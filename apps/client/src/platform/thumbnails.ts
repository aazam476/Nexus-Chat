/**
 * An image's dimensions and a small thumbnail, made on the sender's device
 * before encryption (spec §4.5), so recipients can show a preview without
 * downloading the full image.
 */
import { ImageManipulator, SaveFormat } from "expo-image-manipulator";

import type { PickedFile } from "../messaging/attachmentTransfer";
import { fromBase64 } from "../messaging/engine/bytes";

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
