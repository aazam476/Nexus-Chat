/**
 * iOS and Android: the Keychain and the Keystore, through expo-secure-store
 * (spec §5.3). Values are only readable by this app on this device.
 */
import * as SecureStore from "expo-secure-store";

import type { SecureStorage } from "./storage";

/** The platform's secure storage. */
export function createSecureStorage(): SecureStorage {
  return {
    get: async (key) => (await SecureStore.getItemAsync(key)) ?? undefined,
    set: (key, value) => SecureStore.setItemAsync(key, value),
    remove: (key) => SecureStore.deleteItemAsync(key),
  };
}
