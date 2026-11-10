/**
 * Wraps the web build's database key with the operating system's secret
 * store (Electron `safeStorage`: Keychain on macOS, DPAPI on Windows, the
 * Secret Service on Linux), as ADR 0013 asks for the desktop app. The page
 * sends 32 key bytes and gets back bytes only this OS account can unwrap.
 *
 * On Linux without a keyring Electron falls back to a fixed, public key
 * ("basic_text"), which protects nothing; then this declines, and the page
 * keeps its non-extractable WebCrypto key instead.
 */

/** The part of Electron's safeStorage this uses. */
export interface SafeStorage {
  isEncryptionAvailable(): boolean;
  getSelectedStorageBackend?(): string;
  encryptString(plainText: string): Buffer;
  decryptString(encrypted: Buffer): string;
}

const keyBytes = 32;

function usable(storage: SafeStorage): boolean {
  return storage.isEncryptionAvailable() && storage.getSelectedStorageBackend?.() !== "basic_text";
}

/** The key wrapped by the OS, or undefined where the OS offers no real protection. */
export function protectKey(storage: SafeStorage, key: unknown): Uint8Array | undefined {
  if (!(key instanceof Uint8Array) || key.length !== keyBytes) {
    throw new TypeError("expected a 32-byte key");
  }
  if (!usable(storage)) {
    return undefined;
  }
  return new Uint8Array(storage.encryptString(Buffer.from(key).toString("base64")));
}

/** The key that `protectKey` wrapped. */
export function unprotectKey(storage: SafeStorage, sealed: unknown): Uint8Array {
  if (!(sealed instanceof Uint8Array)) {
    throw new TypeError("expected wrapped key bytes");
  }
  const key = Buffer.from(storage.decryptString(Buffer.from(sealed)), "base64");
  if (key.length !== keyBytes) {
    throw new Error("the unwrapped key is not 32 bytes");
  }
  return new Uint8Array(key);
}
