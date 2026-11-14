/**
 * Property-based round-trips (spec §7.1) for every encoding this package
 * defines: whatever the input, decoding what was encoded gives it back, and
 * cutting or extending the bytes is refused.
 */
import fc from "fast-check";

import { decryptStream, encryptStream } from "./attachments/attachments.js";
import { lengthPrefixed, readFields } from "./encoding.js";
import { decodeHistoryBundle, encodeHistoryBundle } from "./history/bundle.js";
import { decodeDeviceCredential, encodeDeviceCredential } from "./identity/credential.js";

const bytes = fc.uint8Array({ maxLength: 64 });

async function collect(source: AsyncIterable<Uint8Array>): Promise<Uint8Array> {
  const parts: Uint8Array[] = [];
  for await (const part of source) {
    parts.push(part);
  }
  return Uint8Array.from(parts.flatMap((part) => [...part]));
}

async function* inChunks(data: Uint8Array, sizes: number[]): AsyncGenerator<Uint8Array> {
  let offset = 0;
  for (const size of sizes.length === 0 ? [data.length] : sizes) {
    await Promise.resolve();
    yield data.slice(offset, offset + size);
    offset += size;
  }
  yield data.slice(offset);
}

describe("encodings round-trip", () => {
  it("length-prefixed fields", () => {
    fc.assert(
      fc.property(fc.array(bytes, { maxLength: 8 }), (fields) => {
        const encoded = lengthPrefixed(...fields);
        expect(readFields(encoded, fields.length)).toEqual(fields);
        if (encoded.length > 0) {
          expect(readFields(encoded.slice(0, -1), fields.length)).toBeUndefined();
        }
        expect(readFields(new Uint8Array([...encoded, 0]), fields.length)).toBeUndefined();
      }),
    );
  });

  it("device credentials", () => {
    fc.assert(
      fc.property(fc.string(), bytes, (identity, certificate) => {
        const encoded = encodeDeviceCredential({ identity, certificate });
        expect(decodeDeviceCredential(encoded)).toEqual({ identity, certificate });
        expect(decodeDeviceCredential(encoded.slice(0, -1))).toBeUndefined();
      }),
    );
  });

  it("history bundles", () => {
    const entry = fc.record({
      groupId: fc.uint8Array({ minLength: 16, maxLength: 16 }),
      epoch: fc.bigInt({ min: 0n, max: 2n ** 64n - 1n }),
      key: fc.uint8Array({ minLength: 32, maxLength: 32 }),
    });
    fc.assert(
      fc.property(bytes, fc.array(entry, { maxLength: 6 }), (channelId, entries) => {
        const encoded = encodeHistoryBundle(channelId, entries);
        expect(decodeHistoryBundle(encoded)).toEqual({ channelId, entries });
        expect(decodeHistoryBundle(encoded.slice(0, -1))).toBeUndefined();
      }),
    );
  });

  it("attachment streams, however the bytes are split on the way", async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.uint8Array({ maxLength: 4096 }),
        fc.array(fc.integer({ min: 0, max: 1500 }), { maxLength: 5 }),
        fc.array(fc.integer({ min: 1, max: 1500 }), { maxLength: 5 }),
        async (file, writeSizes, readSizes) => {
          const stream = await encryptStream(inChunks(file, writeSizes));
          const ciphertext = await collect(stream.ciphertext);
          const plaintext = await collect(decryptStream(inChunks(ciphertext, readSizes), stream.secrets()));
          expect(plaintext).toEqual(file);
        },
      ),
      { numRuns: 30 },
    );
  });
});
