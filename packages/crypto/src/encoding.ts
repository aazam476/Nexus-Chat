/**
 * The one byte layout used for everything this package signs or seals
 * besides MLS itself: each field preceded by its length as 4 bytes,
 * big-endian. Fixed field order plus lengths make every encoding
 * unambiguous, so two different values never produce the same bytes.
 */

/** `parts`, each preceded by its 4-byte length. */
export function lengthPrefixed(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((total, part) => total + 4 + part.length, 0));
  const view = new DataView(out.buffer);
  let offset = 0;
  for (const part of parts) {
    view.setUint32(offset, part.length);
    out.set(part, offset + 4);
    offset += 4 + part.length;
  }
  return out;
}

/** Exactly `count` length-prefixed fields filling `bytes`, or undefined. */
export function readFields(bytes: Uint8Array, count: number): Uint8Array[] | undefined {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const fields: Uint8Array[] = [];
  let offset = 0;
  while (fields.length < count) {
    if (offset + 4 > bytes.length) {
      return undefined;
    }
    const end = offset + 4 + view.getUint32(offset);
    if (end > bytes.length) {
      return undefined;
    }
    fields.push(bytes.slice(offset + 4, end));
    offset = end;
  }
  return offset === bytes.length ? fields : undefined;
}

/** A number as 8 bytes, big-endian. */
export function uint64(value: bigint): Uint8Array {
  const out = new Uint8Array(8);
  new DataView(out.buffer).setBigUint64(0, value);
  return out;
}

/** Text as UTF-8. */
export const utf8 = (value: string): Uint8Array => new TextEncoder().encode(value);
