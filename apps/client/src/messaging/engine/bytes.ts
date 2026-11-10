/**
 * The API sends bytes as base64 strings (OpenAPI `format: byte`); the
 * crypto code works with Uint8Array. These convert between the two.
 * Hermes, browsers and Node all provide atob and btoa.
 */

/** Bytes as standard base64. */
export function toBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary);
}

/** Standard base64 as bytes. */
export function fromBase64(text: string): Uint8Array {
  return Uint8Array.from(atob(text), (character) => character.charCodeAt(0));
}

/** Text as UTF-8 bytes. */
export const utf8 = (value: string): Uint8Array => new TextEncoder().encode(value);
