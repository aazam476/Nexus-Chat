import sodium from "libsodium-wrappers-sumo";

import {
  AttachmentError,
  decryptFile,
  decryptStream,
  encryptFile,
  encryptStream,
  oneShotLimit,
  schemeFor,
  streamPieceSize,
  type AttachmentSecrets,
} from "./attachments.js";

/** Deterministic bytes, so a mix-up of pieces shows. */
const bytes = (length: number): Uint8Array<ArrayBuffer> =>
  Uint8Array.from({ length }, (_, index) => (index * 31 + 7) % 251);

/** `data` delivered in chunks of `size` bytes, as a file read or download would. */
async function* chunks(data: Uint8Array, size: number): AsyncGenerator<Uint8Array> {
  for (let start = 0; start < data.length; start += size) {
    await Promise.resolve(); // Each chunk arrives later, like a real read.
    yield data.slice(start, start + size);
  }
}

async function collect(source: AsyncIterable<Uint8Array>): Promise<Uint8Array> {
  const parts: Uint8Array[] = [];
  for await (const part of source) {
    parts.push(part);
  }
  const all = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    all.set(part, offset);
    offset += part.length;
  }
  return all;
}

async function encrypted(data: Uint8Array): Promise<[Uint8Array, AttachmentSecrets]> {
  const stream = await encryptStream(chunks(data, 100_003));
  const ciphertext = await collect(stream.ciphertext);
  return [ciphertext, stream.secrets()];
}

const sealed = streamPieceSize + 17;

describe("attachments up to 64 MiB", () => {
  it("are encrypted in one piece and come back unchanged", async () => {
    const file = bytes(5000);
    const { ciphertext, secrets } = await encryptFile(file);

    expect(secrets.scheme).toBe("aes-256-gcm");
    expect(secrets.key).toHaveLength(32);
    expect(secrets.nonce).toHaveLength(12);
    expect(ciphertext).toHaveLength(5000 + 16);
    expect(await decryptFile(ciphertext, secrets)).toEqual(file);
  });

  it("use a fresh key and nonce for every file", async () => {
    const first = await encryptFile(bytes(10));
    const second = await encryptFile(bytes(10));

    expect(first.secrets.key).not.toEqual(second.secrets.key);
    expect(first.secrets.nonce).not.toEqual(second.secrets.nonce);
  });

  it("are refused when changed, swapped or encrypted as a stream", async () => {
    const { ciphertext, secrets } = await encryptFile(bytes(100));
    const changed = ciphertext.slice();
    changed[0] = (changed[0] ?? 0) ^ 1;
    const other = await encryptFile(bytes(100));

    await expect(decryptFile(changed, secrets)).rejects.toThrow("the file is not the one that was sent");
    // The hash matches but the key is another file's.
    await expect(
      decryptFile(ciphertext, { ...other.secrets, ciphertextSha256: secrets.ciphertextSha256 }),
    ).rejects.toThrow("the file could not be decrypted");
    await expect(decryptFile(ciphertext, { ...secrets, nonce: undefined })).rejects.toThrow(AttachmentError);
    await expect(
      decryptFile(ciphertext, { ...secrets, scheme: "xchacha20poly1305-secretstream" }),
    ).rejects.toThrow("this file was encrypted as a stream");
  });

  it("stop at 64 MiB", async () => {
    expect(schemeFor(oneShotLimit)).toBe("aes-256-gcm");
    expect(schemeFor(oneShotLimit + 1)).toBe("xchacha20poly1305-secretstream");
    await expect(encryptFile(new Uint8Array(oneShotLimit + 1))).rejects.toThrow(RangeError);
  });
});

describe("attachments as a stream", () => {
  it.each([0, 1, streamPieceSize - 1, streamPieceSize, streamPieceSize + 1, 2 * streamPieceSize + 12_345])(
    "come back unchanged at %i bytes",
    async (size) => {
      const file = bytes(size);
      const [ciphertext, secrets] = await encrypted(file);

      expect(secrets.scheme).toBe("xchacha20poly1305-secretstream");
      // The header, then one sealed piece per started MiB (at least one).
      expect(ciphertext).toHaveLength(24 + size + 17 * Math.max(1, Math.ceil(size / streamPieceSize)));
      expect(await collect(decryptStream(chunks(ciphertext, 65_537), secrets))).toEqual(file);
    },
  );

  it("give their secrets only once the whole ciphertext has been read", async () => {
    const stream = await encryptStream(chunks(bytes(10), 10));

    expect(() => stream.secrets()).toThrow("read the whole ciphertext first");
  });

  it("are refused when cut off, extended, changed or reordered", async () => {
    const file = bytes(2 * streamPieceSize + 10);
    const [ciphertext, secrets] = await encrypted(file);
    const read = (data: Uint8Array, with_: AttachmentSecrets = secrets): Promise<Uint8Array> =>
      collect(decryptStream(chunks(data, 1 << 20), with_));
    const changed = ciphertext.slice();
    changed[100] = (changed[100] ?? 0) ^ 1;
    const reordered = new Uint8Array(ciphertext.length);
    reordered.set(ciphertext.subarray(0, 24));
    reordered.set(ciphertext.subarray(24 + sealed, 24 + 2 * sealed), 24);
    reordered.set(ciphertext.subarray(24, 24 + sealed), 24 + sealed);
    reordered.set(ciphertext.subarray(24 + 2 * sealed), 24 + 2 * sealed);
    const extended = new Uint8Array(ciphertext.length + 1);
    extended.set(ciphertext);

    await expect(read(ciphertext.subarray(0, 10))).rejects.toThrow("the file is cut off");
    await expect(read(ciphertext.subarray(0, 24 + 2 * sealed))).rejects.toThrow("the file is cut off");
    // A byte added to the last piece breaks that piece.
    await expect(read(extended)).rejects.toThrow("the file could not be decrypted");
    await expect(read(changed)).rejects.toThrow("the file could not be decrypted");
    await expect(read(reordered)).rejects.toThrow("the file could not be decrypted");
    // Too short to hold even an authentication tag.
    await expect(read(ciphertext.subarray(0, 24 + 5))).rejects.toThrow("the file could not be decrypted");
    await expect(read(ciphertext, { ...secrets, ciphertextSha256: new Uint8Array(32) })).rejects.toThrow(
      "the file is not the one that was sent",
    );
    await expect(read(ciphertext, { ...secrets, scheme: "aes-256-gcm" })).rejects.toThrow(
      "this file was encrypted in one piece",
    );
  });

  it("are refused with anything after the final piece", async () => {
    // A whole number of pieces: the extra bytes arrive as a piece of their own.
    const [ciphertext, secrets] = await encrypted(bytes(streamPieceSize));
    const extended = new Uint8Array(ciphertext.length + 40);
    extended.set(ciphertext);

    await expect(collect(decryptStream(chunks(extended, 4096), secrets))).rejects.toThrow(
      "the file has data after its end",
    );
  });

  it("accept only message and final pieces", async () => {
    await sodium.ready;
    const key = sodium.crypto_secretstream_xchacha20poly1305_keygen();
    const { state, header } = sodium.crypto_secretstream_xchacha20poly1305_init_push(key);
    const piece = sodium.crypto_secretstream_xchacha20poly1305_push(
      state,
      bytes(3),
      null,
      sodium.crypto_secretstream_xchacha20poly1305_TAG_REKEY,
    );
    const ciphertext = new Uint8Array([...header, ...piece]);
    const secrets: AttachmentSecrets = {
      scheme: "xchacha20poly1305-secretstream",
      key,
      ciphertextSha256: new Uint8Array(32),
    };

    await expect(collect(decryptStream(chunks(ciphertext, 1000), secrets))).rejects.toThrow(
      "the file could not be decrypted",
    );
  });
});
