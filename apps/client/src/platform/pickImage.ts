/**
 * Choosing a picture with the system's own picker (expo-image-picker),
 * which needs no broad storage permission.
 */
import { launchImageLibraryAsync, type ImagePickerOptions } from "expo-image-picker";

/**
 * The chosen picture's bytes, or undefined when the user cancelled. In a
 * browser the picker hands over the File itself; reading its `data:`
 * address with fetch would be refused by the page's content security
 * policy, which only lets the app connect to its own instance.
 */
async function pick(options: ImagePickerOptions): Promise<Blob | undefined> {
  const result = await launchImageLibraryAsync(options);
  const asset = result.canceled ? undefined : result.assets[0];
  if (asset === undefined) {
    return undefined;
  }
  return asset.file ?? (await fetch(asset.uri)).blob();
}

/**
 * A picture cropped to a square where the platform offers it; the server
 * takes the centered square either way.
 */
export async function pickSquareImage(): Promise<Blob | undefined> {
  return pick({ mediaTypes: ["images"], allowsEditing: true, aspect: [1, 1], quality: 1 });
}

/**
 * A picture as it is, uncropped and not converted, so a GIF keeps its
 * animation and a PNG its transparency.
 */
export async function pickOriginalImage(): Promise<Blob | undefined> {
  return pick({ mediaTypes: ["images"], allowsEditing: false, quality: 1 });
}
