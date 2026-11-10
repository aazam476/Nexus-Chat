/**
 * Attachment encryption (docs/e2ee-design.md §7). Every file gets its own
 * random 256-bit key, sent inside the (MLS-encrypted) message with the
 * ciphertext's SHA-256, so the server stores only ciphertext.
 *
 * - Files up to 64 MiB: one-shot AES-256-GCM through WebCrypto.
 * - Larger files: libsodium's `crypto_secretstream_xchacha20poly1305`, in
 *   1 MiB pieces, so a phone never holds the whole file. The stream is the
 *   24-byte header followed by one secretstream message per piece; the last
 *   one carries the FINAL tag, so a cut-off file is detected.
 *
 * Both are published constructions used as documented; nothing here
 * invents its own chunking or nonces.
 */
import { sha256 } from "@noble/hashes/sha2.js";
import sodium from "libsodium-wrappers-sumo";

/** Files up to this size are encrypted in one piece. */
export const oneShotLimit = 64 * 1024 * 1024;
/** Plaintext bytes per secretstream message. */
export const streamPieceSize = 1024 * 1024;

/** How a file is encrypted: in one piece, or as a stream of pieces. */
export type AttachmentScheme = "aes-256-gcm" | "xchacha20poly1305-secretstream";

/** What the recipient needs to fetch, check and decrypt a file. */
export interface AttachmentSecrets {
  scheme: AttachmentScheme;
  key: Uint8Array;
  /** The AES-GCM nonce; absent for streams, whose header is in the ciphertext. */
  nonce?: Uint8Array;
  /** SHA-256 of the ciphertext, checked before anything is decrypted. */
  ciphertextSha256: Uint8Array;
}

/** A file that can't be decrypted: wrong key, changed or cut-off bytes. */
export class AttachmentError extends Error {
  override name = "AttachmentError";
}

/** Which scheme a file of `size` bytes is encrypted with. */
export function schemeFor(size: number): AttachmentScheme {
  return size <= oneShotLimit ? "aes-256-gcm" : "xchacha20poly1305-secretstream";
}

/** Bytes in memory WebCrypto accepts (not a SharedArrayBuffer). */
type Bytes = Uint8Array<ArrayBuffer>;

const randomBytes = (length: number): Bytes => crypto.getRandomValues(new Uint8Array(length));

const importKey = (key: Uint8Array, usage: KeyUsage): Promise<CryptoKey> =>
  crypto.subtle.importKey("raw", new Uint8Array(key), "AES-GCM", false, [usage]);

/** Compares hashes of ciphertext, which is public, so timing does not matter. */
const sameBytes = (a: Uint8Array, b: Uint8Array): boolean =>
  a.length === b.length && a.every((byte, index) => byte === b[index]);

/** Encrypts a file of at most 64 MiB in one piece. */
export async function encryptFile(
  plaintext: Bytes,
): Promise<{ ciphertext: Bytes; secrets: AttachmentSecrets }> {
  if (plaintext.length > oneShotLimit) {
    throw new RangeError("files over 64 MiB are encrypted as a stream");
  }
  const key = randomBytes(32);
  const nonce = randomBytes(12);
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, await importKey(key, "encrypt"), plaintext),
  );
  return { ciphertext, secrets: { scheme: "aes-256-gcm", key, nonce, ciphertextSha256: sha256(ciphertext) } };
}

/** Checks and decrypts a file encrypted with `encryptFile`. */
export async function decryptFile(ciphertext: Bytes, secrets: AttachmentSecrets): Promise<Bytes> {
  if (secrets.scheme !== "aes-256-gcm" || secrets.nonce === undefined) {
    throw new AttachmentError("this file was encrypted as a stream");
  }
  if (!sameBytes(sha256(ciphertext), secrets.ciphertextSha256)) {
    throw new AttachmentError("the file is not the one that was sent");
  }
  try {
    return new Uint8Array(
      await crypto.subtle.decrypt(
        { name: "AES-GCM", iv: new Uint8Array(secrets.nonce) },
        await importKey(secrets.key, "decrypt"),
        ciphertext,
      ),
    );
  } catch {
    throw new AttachmentError("the file could not be decrypted");
  }
}

/**
 * Reads `source` in exact lengths: each call returns `length` bytes, or
 * fewer only once the source has ended (an empty result means the end).
 */
function reader(source: AsyncIterable<Uint8Array>): (length: number) => Promise<Uint8Array> {
  const iterator = source[Symbol.asyncIterator]();
  let buffer = new Uint8Array(0);
  let ended = false;
  return async (length) => {
    while (buffer.length < length && !ended) {
      const next = await iterator.next();
      if (next.done === true) {
        ended = true;
      } else {
        const joined = new Uint8Array(buffer.length + next.value.length);
        joined.set(buffer);
        joined.set(next.value, buffer.length);
        buffer = joined;
      }
    }
    const result = buffer.slice(0, length);
    buffer = buffer.slice(length);
    return result;
  };
}

/** A large file being encrypted; read `ciphertext` to the end, then `secrets()`. */
export interface StreamEncryption {
  ciphertext: AsyncIterable<Uint8Array>;
  /** Available once `ciphertext` has been read to the end. */
  secrets: () => AttachmentSecrets;
}

/** The header, then each piece sealed; every byte also goes into `hash`. */
async function* seal(
  source: AsyncIterable<Uint8Array>,
  key: Uint8Array,
  hash: ReturnType<typeof sha256.create>,
): AsyncGenerator<Uint8Array> {
  const { state, header } = sodium.crypto_secretstream_xchacha20poly1305_init_push(key);
  hash.update(header);
  yield header;
  // Read one piece ahead so the last one, possibly empty, gets the FINAL tag.
  const read = reader(source);
  let held = await read(streamPieceSize);
  for (let next = await read(streamPieceSize); next.length > 0; next = await read(streamPieceSize)) {
    const sealed = sodium.crypto_secretstream_xchacha20poly1305_push(
      state,
      held,
      null,
      sodium.crypto_secretstream_xchacha20poly1305_TAG_MESSAGE,
    );
    hash.update(sealed);
    yield sealed;
    held = next;
  }
  const last = sodium.crypto_secretstream_xchacha20poly1305_push(
    state,
    held,
    null,
    sodium.crypto_secretstream_xchacha20poly1305_TAG_FINAL,
  );
  hash.update(last);
  yield last;
}

/** Encrypts a file of any size, piece by piece, as it is read. */
export async function encryptStream(source: AsyncIterable<Uint8Array>): Promise<StreamEncryption> {
  await sodium.ready;
  const key = sodium.crypto_secretstream_xchacha20poly1305_keygen();
  const hash = sha256.create();
  let digest: Uint8Array | undefined;
  async function* ciphertext(): AsyncGenerator<Uint8Array> {
    yield* seal(source, key, hash);
    digest = hash.digest();
  }
  return {
    ciphertext: ciphertext(),
    secrets: () => {
      if (digest === undefined) {
        throw new Error("read the whole ciphertext first");
      }
      return { scheme: "xchacha20poly1305-secretstream", key, ciphertextSha256: digest };
    },
  };
}

/**
 * Decrypts a stream piece by piece. Each piece is authenticated before it
 * is yielded, but the file is only known to be complete, unchanged and in
 * order when the iteration ends without an error: callers write pieces to
 * a temporary file and keep it only then.
 */
export async function* decryptStream(
  source: AsyncIterable<Uint8Array>,
  secrets: AttachmentSecrets,
): AsyncGenerator<Uint8Array> {
  if (secrets.scheme !== "xchacha20poly1305-secretstream") {
    throw new AttachmentError("this file was encrypted in one piece");
  }
  await sodium.ready;
  const headerBytes = sodium.crypto_secretstream_xchacha20poly1305_HEADERBYTES;
  const sealedSize = streamPieceSize + sodium.crypto_secretstream_xchacha20poly1305_ABYTES;
  const hash = sha256.create();
  const read = reader(source);
  const header = await read(headerBytes);
  if (header.length !== headerBytes) {
    throw new AttachmentError("the file is cut off");
  }
  hash.update(header);
  const state = sodium.crypto_secretstream_xchacha20poly1305_init_pull(header, secrets.key);
  let finished = false;
  for (let sealed = await read(sealedSize); sealed.length > 0; sealed = await read(sealedSize)) {
    if (finished) {
      throw new AttachmentError("the file has data after its end");
    }
    hash.update(sealed);
    const opened = open(state, sealed);
    finished = opened.tag === sodium.crypto_secretstream_xchacha20poly1305_TAG_FINAL;
    yield opened.message;
  }
  if (!finished) {
    throw new AttachmentError("the file is cut off");
  }
  if (!sameBytes(hash.digest(), secrets.ciphertextSha256)) {
    throw new AttachmentError("the file is not the one that was sent");
  }
}

function open(
  state: ReturnType<typeof sodium.crypto_secretstream_xchacha20poly1305_init_pull>,
  sealed: Uint8Array,
): { message: Uint8Array; tag: number } {
  let opened: { message: Uint8Array; tag: number } | false;
  try {
    opened = sodium.crypto_secretstream_xchacha20poly1305_pull(state, sealed, null);
  } catch {
    opened = false;
  }
  const allowed = [
    sodium.crypto_secretstream_xchacha20poly1305_TAG_MESSAGE,
    sodium.crypto_secretstream_xchacha20poly1305_TAG_FINAL,
  ];
  if (opened === false || !allowed.includes(opened.tag)) {
    throw new AttachmentError("the file could not be decrypted");
  }
  return opened;
}
