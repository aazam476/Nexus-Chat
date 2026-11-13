/**
 * Which version of a picked file goes out (ADR 0014): a photo's compressed
 * copy, unless the sender chose the original or the copy is not smaller.
 */
import type { PickedFile } from "./attachmentTransfer";

/** The file as it will be encrypted and uploaded. */
export function versionToSend(file: PickedFile): PickedFile {
  const { compressed, sendOriginal, ...original } = file;
  if (compressed === undefined || sendOriginal === true || compressed.bytes.length >= file.bytes.length) {
    return original;
  }
  return {
    ...original,
    name: `${file.name.replace(/\.[^.]*$/, "")}.webp`,
    mimeType: compressed.mimeType,
    bytes: compressed.bytes,
    width: compressed.width,
    height: compressed.height,
  };
}
