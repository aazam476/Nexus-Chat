/**
 * Browsers and Electron: values live in IndexedDB, encrypted with AES-GCM
 * under a WebCrypto key that is generated with `extractable: false` (spec
 * §5.3). The key itself is stored in IndexedDB as a CryptoKey object, which
 * script can use but never read out, so a copied profile or a script
 * dumping storage gets ciphertext only.
 *
 * Browsers only offer WebCrypto on HTTPS and localhost. Elsewhere values
 * stay in memory, and the HTTPS warning explains why logins do not last.
 */
import { createMemoryStorage, type SecureStorage } from "./storage";
import { openStore, type KeyValueStore } from "./indexedDb";

const keyName = "storage-key";
const ivLength = 12;

/** The browser's encrypted storage, or memory where that is unavailable. */
export function createSecureStorage(): SecureStorage {
  if (
    typeof indexedDB === "undefined" ||
    typeof crypto === "undefined" ||
    typeof crypto.subtle === "undefined"
  ) {
    return createMemoryStorage();
  }
  return createEncryptedStorage(openStore());
}

/** Encrypted storage on top of `store`; exported for tests. */
export function createEncryptedStorage(store: KeyValueStore): SecureStorage {
  let key: Promise<CryptoKey> | undefined;
  const storageKey = (): Promise<CryptoKey> => {
    key ??= loadOrCreateKey(store);
    return key;
  };
  return {
    get: async (name) => {
      const sealed = await store.get(name);
      if (!(sealed instanceof Uint8Array)) {
        return undefined;
      }
      try {
        const plaintext = await crypto.subtle.decrypt(
          { name: "AES-GCM", iv: sealed.slice(0, ivLength) },
          await storageKey(),
          sealed.slice(ivLength),
        );
        return new TextDecoder().decode(plaintext);
      } catch {
        // Sealed with a key the browser no longer has (site data partly
        // cleared): the value is unreadable, so forget it.
        await store.delete(name);
        return undefined;
      }
    },
    set: async (name, value) => {
      const iv = crypto.getRandomValues(new Uint8Array(ivLength));
      const ciphertext = await crypto.subtle.encrypt(
        { name: "AES-GCM", iv },
        await storageKey(),
        new TextEncoder().encode(value),
      );
      const sealed = new Uint8Array(ivLength + ciphertext.byteLength);
      sealed.set(iv);
      sealed.set(new Uint8Array(ciphertext), ivLength);
      await store.put(name, sealed);
    },
    remove: (name) => store.delete(name),
  };
}

async function loadOrCreateKey(store: KeyValueStore): Promise<CryptoKey> {
  const existing = await store.get(keyName);
  if (existing instanceof CryptoKey) {
    return existing;
  }
  const created = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, [
    "encrypt",
    "decrypt",
  ]);
  await store.put(keyName, created);
  return created;
}
