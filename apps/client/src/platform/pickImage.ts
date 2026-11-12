/**
 * Choosing a picture with the system's own picker (expo-image-picker),
 * which needs no broad storage permission. The picker crops to a square
 * where the platform offers it; the server takes the centered square
 * either way.
 */
import { launchImageLibraryAsync } from "expo-image-picker";

/** The chosen picture's bytes, or undefined when the user cancelled. */
export async function pickSquareImage(): Promise<Blob | undefined> {
  const result = await launchImageLibraryAsync({
    mediaTypes: ["images"],
    allowsEditing: true,
    aspect: [1, 1],
    quality: 1,
  });
  const asset = result.canceled ? undefined : result.assets[0];
  if (asset === undefined) {
    return undefined;
  }
  return (await fetch(asset.uri)).blob();
}
