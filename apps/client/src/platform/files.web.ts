/**
 * Files in the browser and the desktop app: choosing one to attach,
 * reading it, making an image thumbnail before encryption, and saving a
 * received file through the browser's download.
 */
import { getDocumentAsync } from "expo-document-picker";
import { launchImageLibraryAsync } from "expo-image-picker";

import type { PickedFile } from "../messaging/attachmentTransfer";
import { prepareImage } from "./thumbnails";

async function readBytes(uri: string): Promise<Uint8Array<ArrayBuffer>> {
  return new Uint8Array(await (await fetch(uri)).arrayBuffer());
}

/** A picked file with its image details, when it is an image. */
async function picked(uri: string, name: string, mimeType: string): Promise<PickedFile> {
  const file = { name, mimeType, bytes: await readBytes(uri) };
  return mimeType.startsWith("image/") ? prepareImage(uri, file) : file;
}

/** Any file, through the browser's file chooser; undefined if cancelled. */
export async function pickDocument(): Promise<PickedFile | undefined> {
  const result = await getDocumentAsync({});
  const asset = result.canceled ? undefined : result.assets[0];
  return asset === undefined
    ? undefined
    : picked(asset.uri, asset.name, asset.mimeType ?? "application/octet-stream");
}

/** A photo or video, through the browser's file chooser. */
export async function pickMedia(): Promise<PickedFile | undefined> {
  const result = await launchImageLibraryAsync({ mediaTypes: ["images", "videos"], quality: 1 });
  const asset = result.canceled ? undefined : result.assets[0];
  if (asset === undefined) {
    return undefined;
  }
  const mimeType = asset.mimeType ?? (asset.type === "video" ? "video/mp4" : "image/jpeg");
  return picked(asset.uri, asset.fileName ?? "photo", mimeType);
}

/** Saves a received file through the browser's download. */
export function saveReceivedFile(
  name: string,
  mimeType: string,
  bytes: Uint8Array<ArrayBuffer>,
): Promise<void> {
  const url = URL.createObjectURL(new Blob([bytes], { type: mimeType }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  // Give the browser a moment to start the download before letting go.
  setTimeout(() => {
    URL.revokeObjectURL(url);
  }, 10_000);
  return Promise.resolve();
}
