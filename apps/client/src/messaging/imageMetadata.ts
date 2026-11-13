/**
 * Metadata in images the user sends (ADR 0014): location, camera and
 * editing details are taken out of JPEG, PNG and WebP files before
 * encryption, without touching the picture itself, so even an original
 * sent at full quality carries only the image. Anything this cannot read
 * is sent as it is.
 */

const view = (bytes: Uint8Array): DataView => new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
const ascii = (bytes: Uint8Array, at: number, length: number): string =>
  String.fromCharCode(...bytes.subarray(at, at + length));

function concat(parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
  const out = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let at = 0;
  for (const part of parts) {
    out.set(part, at);
    at += part.length;
  }
  return out;
}

/** JPEG without APP1 (Exif, XMP), APP13 (IPTC) and comment segments. */
function stripJpeg(bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
  const kept: Uint8Array[] = [bytes.subarray(0, 2)];
  let at = 2;
  while (at + 4 <= bytes.length && bytes[at] === 0xff) {
    const marker = bytes[at + 1] ?? 0;
    if (marker === 0xda) {
      break; // Start of scan: the rest is image data.
    }
    const end = at + 2 + view(bytes).getUint16(at + 2);
    if (end > bytes.length) {
      return bytes;
    }
    if (marker !== 0xe1 && marker !== 0xed && marker !== 0xfe) {
      kept.push(bytes.subarray(at, end));
    }
    at = end;
  }
  kept.push(bytes.subarray(at));
  return concat(kept);
}

const pngMetadata = new Set(["eXIf", "tEXt", "iTXt", "zTXt", "tIME"]);

/** PNG without its text, time and Exif chunks. */
function stripPng(bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
  const kept: Uint8Array[] = [bytes.subarray(0, 8)];
  let at = 8;
  while (at + 12 <= bytes.length) {
    const end = at + 12 + view(bytes).getUint32(at);
    if (end > bytes.length) {
      return bytes;
    }
    if (!pngMetadata.has(ascii(bytes, at + 4, 4))) {
      kept.push(bytes.subarray(at, end));
    }
    at = end;
  }
  return concat(kept);
}

/** WebP without EXIF and XMP chunks, with its header flags and size fixed. */
function stripWebp(bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
  const kept: Uint8Array[] = [];
  let at = 12;
  while (at + 8 <= bytes.length) {
    const size = view(bytes).getUint32(at + 4, true);
    const end = at + 8 + size + (size % 2);
    if (end > bytes.length) {
      return bytes;
    }
    const chunk = bytes.slice(at, end);
    if (ascii(chunk, 0, 4) === "VP8X") {
      chunk[8] = (chunk[8] ?? 0) & ~0x0c; // Clear the EXIF and XMP flags.
    }
    if (!["EXIF", "XMP "].includes(ascii(chunk, 0, 4))) {
      kept.push(chunk);
    }
    at = end;
  }
  const body = concat(kept);
  const header = bytes.slice(0, 12);
  view(header).setUint32(4, body.length + 4, true);
  return concat([header, body]);
}

/** The image without its metadata; other files, and unreadable ones, unchanged. */
export function stripMetadata(mimeType: string, bytes: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
  if (mimeType === "image/jpeg" && bytes[0] === 0xff && bytes[1] === 0xd8) {
    return stripJpeg(bytes);
  }
  if (mimeType === "image/png" && ascii(bytes, 1, 3) === "PNG") {
    return stripPng(bytes);
  }
  if (mimeType === "image/webp" && ascii(bytes, 0, 4) === "RIFF" && ascii(bytes, 8, 4) === "WEBP") {
    return stripWebp(bytes);
  }
  return bytes;
}

/**
 * Whether an image may be re-encoded lossily (ADR 0014): photos, but not
 * animations or images with transparency, which would lose what they are.
 */
export function compressible(mimeType: string, bytes: Uint8Array): boolean {
  switch (mimeType) {
    case "image/jpeg":
    case "image/heic":
    case "image/heif":
      return true;
    case "image/png":
      // Colour types 4 and 6 have alpha; a tRNS chunk makes colours transparent.
      return (
        bytes[25] !== 4 && bytes[25] !== 6 && !ascii(bytes, 0, Math.min(bytes.length, 4096)).includes("tRNS")
      );
    case "image/webp":
      // Lossy WebP without the animation or alpha flags of an extended header.
      return (
        ascii(bytes, 12, 4) === "VP8 " || (ascii(bytes, 12, 4) === "VP8X" && ((bytes[20] ?? 0) & 0x12) === 0)
      );
    default:
      return false;
  }
}
