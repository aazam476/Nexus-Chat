/**
 * The storage used where no platform file applies (tests and type
 * checking). The real ones are `platformStorage.native.ts` for iOS and
 * Android and `platformStorage.web.ts` for browsers and Electron, chosen by
 * the bundler.
 */
import { createMemoryStorage, type SecureStorage } from "./storage";

/** Storage for this platform. */
export function createSecureStorage(): SecureStorage {
  return createMemoryStorage();
}
