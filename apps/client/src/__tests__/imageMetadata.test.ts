import { fromBase64 } from "../messaging/engine/bytes";
import { compressible, stripMetadata } from "../messaging/imageMetadata";

// 64×48 images saved with a camera name in Exif, a comment, a location in
// PNG text or XMP: what a phone's photo can carry.
const files = {
  jpeg: "/9j/4AAQSkZJRgABAQAAAQABAAD/4QAsRXhpZgAATU0AKgAAAAgAAQEPAAIAAAAKAAAAGgAAAABTZWNyZXRDYW0A//4AEXByaXZhdGUgY29tbWVudP/bAEMACAYGBwYFCAcHBwkJCAoMFA0MCwsMGRITDxQdGh8eHRocHCAkLicgIiwjHBwoNyksMDE0NDQfJzk9ODI8LjM0Mv/bAEMBCQkJDAsMGA0NGDIhHCEyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMv/AABEIADAAQAMBIgACEQEDEQH/xAAfAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgv/xAC1EAACAQMDAgQDBQUEBAAAAX0BAgMABBEFEiExQQYTUWEHInEUMoGRoQgjQrHBFVLR8CQzYnKCCQoWFxgZGiUmJygpKjQ1Njc4OTpDREVGR0hJSlNUVVZXWFlaY2RlZmdoaWpzdHV2d3h5eoOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4eLj5OXm5+jp6vHy8/T19vf4+fr/xAAfAQADAQEBAQEBAQEBAAAAAAAAAQIDBAUGBwgJCgv/xAC1EQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/AOQooorxT9MCiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooAKKKKACiiigAooooA//2Q==",
  png: "iVBORw0KGgoAAAANSUhEUgAAAEAAAAAwCAIAAAAuKetIAAAAD3RFWHRMb2NhdGlvbgBCZXJsaW5PPiCLAAAAUklEQVR4nO3PMQ0AIADAMEAI/qUgCxEcDcmqYJtn7/GzpQNeNaA1oDWgNaA1oDWgNaA1oDWgNaA1oDWgNaA1oDWgNaA1oDWgNaA1oDWgNaA1oF2vFwFk2XER3gAAAABJRU5ErkJggg==",
  webp: "UklGRrQAAABXRUJQVlA4WAoAAAAMAAAAPwAALwAAVlA4IEIAAACQAwCdASpAADAAPm02mEkkIyKhIqgAgA2JZwB2AAAqszoTaYoAAP7wOEP/4ta3C0sP//tLP/qWf/Us/xiVFKscAABFWElGJAAAAE1NACoAAAAIAAEBDwACAAAACgAAABoAAAAAU2VjcmV0Q2FtAFhNUCAgAAAAPHg6eG1wbWV0YT5TZWNyZXRYTVA8L3g6eG1wbWV0YT4=",
};
const bytes = (base64: string): Uint8Array<ArrayBuffer> => new Uint8Array(fromBase64(base64));
const text = (data: Uint8Array): string => String.fromCharCode(...data);
const ascii = (value: string): Uint8Array => new TextEncoder().encode(value);
const secrets = /SecretCam|private comment|Berlin|SecretXMP/;

describe("taking metadata out of images", () => {
  it.each([
    ["image/jpeg", files.jpeg],
    ["image/png", files.png],
    ["image/webp", files.webp],
  ])("strips %s without touching the picture", (mimeType, base64) => {
    const original = bytes(base64);
    expect(text(original)).toMatch(secrets);
    const stripped = stripMetadata(mimeType, original);
    expect(text(stripped)).not.toMatch(secrets);
    expect(stripped.length).toBeLessThan(original.length);
    // Stripping twice changes nothing more.
    expect(stripMetadata(mimeType, stripped)).toEqual(stripped);
  });

  it("keeps the WebP header consistent", () => {
    const stripped = stripMetadata("image/webp", bytes(files.webp));
    const view = new DataView(stripped.buffer);
    expect(view.getUint32(4, true)).toBe(stripped.length - 8);
    expect((stripped[20] ?? 0) & 0x0c).toBe(0);
  });

  it("leaves other files and damaged images as they are", () => {
    const pdf = new Uint8Array([0x25, 0x50, 0x44, 0x46]);
    expect(stripMetadata("application/pdf", pdf)).toBe(pdf);
    expect(stripMetadata("image/jpeg", pdf)).toBe(pdf);
    for (const [mimeType, base64] of [
      ["image/jpeg", files.jpeg],
      ["image/png", files.png],
      ["image/webp", files.webp],
    ] as const) {
      // Cut inside a segment or chunk.
      const cut = bytes(base64).slice(0, 26);
      expect(stripMetadata(mimeType, cut)).toBe(cut);
    }
  });
});

describe("which images may be compressed", () => {
  const webp = (chunk: string, flags = 0): Uint8Array => {
    const data = new Uint8Array(30);
    data.set(ascii("RIFF"), 0);
    data.set(ascii("WEBP"), 8);
    data.set(ascii(chunk), 12);
    data[20] = flags;
    return data;
  };

  it("compresses photos but keeps animations and transparency", () => {
    expect(compressible("image/jpeg", new Uint8Array(4))).toBe(true);
    expect(compressible("image/heic", new Uint8Array(4))).toBe(true);
    expect(compressible("image/png", bytes(files.png))).toBe(true);
    const withAlpha = bytes(files.png);
    withAlpha[25] = 6;
    expect(compressible("image/png", withAlpha)).toBe(false);
    const greyAlpha = bytes(files.png);
    greyAlpha[25] = 4;
    expect(compressible("image/png", greyAlpha)).toBe(false);
    const palette = new Uint8Array([...bytes(files.png), ...ascii("tRNS")]);
    expect(compressible("image/png", palette)).toBe(false);
    expect(compressible("image/webp", webp("VP8 "))).toBe(true);
    expect(compressible("image/webp", webp("VP8X"))).toBe(true);
    expect(compressible("image/webp", webp("VP8X", 0x02))).toBe(false);
    expect(compressible("image/webp", webp("VP8X", 0x10))).toBe(false);
    expect(compressible("image/webp", webp("VP8L"))).toBe(false);
    expect(compressible("image/gif", new Uint8Array(4))).toBe(false);
    expect(compressible("video/mp4", new Uint8Array(4))).toBe(false);
  });
});
