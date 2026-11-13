/**
 * Images on the sender's device, before encryption: their dimensions and a
 * small thumbnail (spec §4.5), so recipients can show a preview without
 * downloading the full image, and for photos a compressed copy without
 * metadata (ADR 0014).
 */
import { ImageManipulator, SaveFormat } from "expo-image-manipulator";

import type { PickedFile } from "../messaging/attachmentTransfer";
import { fromBase64 } from "../messaging/engine/bytes";
import { compressible, stripMetadata } from "../messaging/imageMetadata";

/** The longest side of a thumbnail, in pixels. */
export const thumbnailSize = 320;

/** The image's size and a JPEG thumbnail of it; nothing if it can't be read. */
export async function describeImage(
  uri: string,
): Promise<Pick<PickedFile, "width" | "height" | "thumbnail">> {
  try {
    const original = await ImageManipulator.manipulate(uri).renderAsync();
    const scale = thumbnailSize / Math.max(original.width, original.height, thumbnailSize);
    const resized = await ImageManipulator.manipulate(uri)
      .resize({ width: Math.round(original.width * scale) })
      .renderAsync();
    const small = await resized.saveAsync({ format: SaveFormat.JPEG, compress: 0.7, base64: true });
    return {
      width: original.width,
      height: original.height,
      thumbnail: {
        bytes: new Uint8Array(fromBase64(small.base64 ?? "")),
        mimeType: "image/jpeg",
        width: small.width,
        height: small.height,
      },
    };
  } catch {
    return {};
  }
}

/** The longest side of a compressed photo, in pixels (ADR 0014). */
export const compressedSize = 2560;

/** A photo re-encoded as WebP at quality 82, at most 2560 pixels long, without metadata. */
async function compressImage(uri: string, width: number, height: number): Promise<PickedFile["compressed"]> {
  try {
    const scale = Math.min(1, compressedSize / Math.max(width, height));
    const context = ImageManipulator.manipulate(uri);
    if (scale < 1) {
      context.resize({ width: Math.round(width * scale) });
    }
    const image = await (
      await context.renderAsync()
    ).saveAsync({
      format: SaveFormat.WEBP,
      compress: 0.82,
      base64: true,
    });
    return {
      bytes: new Uint8Array(fromBase64(image.base64 ?? "")),
      mimeType: "image/webp",
      width: image.width,
      height: image.height,
    };
  } catch {
    return undefined;
  }
}

/** A HEIC photo as a full-quality JPEG, which has no metadata after re-encoding. */
async function heicAsJpeg(uri: string): Promise<Pick<PickedFile, "bytes" | "mimeType"> | undefined> {
  try {
    const image = await (
      await ImageManipulator.manipulate(uri).renderAsync()
    ).saveAsync({
      format: SaveFormat.JPEG,
      compress: 1,
      base64: true,
    });
    return { bytes: new Uint8Array(fromBase64(image.base64 ?? "")), mimeType: "image/jpeg" };
  } catch {
    return undefined;
  }
}

/**
 * A picked image ready to send: its size and thumbnail, its metadata taken
 * out, and for photos a compressed copy the sender may send instead.
 */
export async function prepareImage(uri: string, file: PickedFile): Promise<PickedFile> {
  const described = await describeImage(uri);
  const heic = file.mimeType === "image/heic" || file.mimeType === "image/heif";
  const converted = heic ? await heicAsJpeg(uri) : undefined;
  const original = converted ?? { bytes: stripMetadata(file.mimeType, file.bytes), mimeType: file.mimeType };
  const compressed =
    compressible(file.mimeType, file.bytes) && described.width !== undefined && described.height !== undefined
      ? await compressImage(uri, described.width, described.height)
      : undefined;
  return {
    ...file,
    ...original,
    ...(converted === undefined ? {} : { name: `${file.name.replace(/\.[^.]*$/, "")}.jpg` }),
    ...described,
    ...(compressed === undefined ? {} : { compressed }),
  };
}
