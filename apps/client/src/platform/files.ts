/**
 * Files on iOS and Android: choosing one to attach, reading it, making an
 * image thumbnail before encryption, and handing a received file to the
 * system share sheet. The web version is files.web.ts.
 */
import { getDocumentAsync } from "expo-document-picker";
import { File, Paths } from "expo-file-system";
import { launchImageLibraryAsync } from "expo-image-picker";
import { shareAsync } from "expo-sharing";

import type { PickedFile } from "../messaging/attachmentTransfer";
import { prepareImage } from "./thumbnails";

async function readBytes(uri: string): Promise<Uint8Array<ArrayBuffer>> {
  return new Uint8Array(await new File(uri).bytes());
}

/** Any file, through the system's document picker; undefined if cancelled. */
export async function pickDocument(): Promise<PickedFile | undefined> {
  const result = await getDocumentAsync({ copyToCacheDirectory: true });
  const asset = result.canceled ? undefined : result.assets[0];
  if (asset === undefined) {
    return undefined;
  }
  const file = {
    name: asset.name,
    mimeType: asset.mimeType ?? "application/octet-stream",
    bytes: await readBytes(asset.uri),
  };
  return file.mimeType.startsWith("image/") ? prepareImage(asset.uri, file) : file;
}

/** A photo or video from the library, through the system picker. */
export async function pickMedia(): Promise<PickedFile | undefined> {
  const result = await launchImageLibraryAsync({ mediaTypes: ["images", "videos"], quality: 1 });
  const asset = result.canceled ? undefined : result.assets[0];
  if (asset === undefined) {
    return undefined;
  }
  const mimeType = asset.mimeType ?? (asset.type === "video" ? "video/mp4" : "image/jpeg");
  const file = { name: asset.fileName ?? "photo", mimeType, bytes: await readBytes(asset.uri) };
  return asset.type === "image" ? prepareImage(asset.uri, file) : file;
}

/** Writes a received file to the cache and offers it to other apps. */
export async function saveReceivedFile(
  name: string,
  mimeType: string,
  bytes: Uint8Array<ArrayBuffer>,
): Promise<void> {
  const file = new File(Paths.cache, name.replace(/[/\\]/g, "_"));
  await file.write(bytes);
  await shareAsync(file.uri, { mimeType });
}
