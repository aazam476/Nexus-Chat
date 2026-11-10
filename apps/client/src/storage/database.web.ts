/**
 * Browsers and Electron: database records in IndexedDB, each sealed with
 * AES-256-GCM (ADR 0013, spec §5.3), with the collection and ID bound as
 * additional data so a record can't be moved to another place unnoticed.
 *
 * The data key is a non-extractable WebCrypto key kept in IndexedDB, which
 * script can use but never read. In the desktop app it is instead random
 * bytes wrapped by the operating system (Electron `safeStorage`: Keychain,
 * DPAPI or the Secret Service) and imported as non-extractable at start,
 * which ties the database to the OS account as well.
 */
import { createMemoryDatabase, type LocalDatabase } from "./localDatabase";
import { openStore, type KeyValueStore } from "./indexedDb";

const keyName = "\u0000key";
const protectedKeyName = "\u0000protected-key";
const ivLength = 12;
const separator = "\u0001";

/** What the desktop shell offers (apps/desktop/src/preload.ts). */
interface DesktopKeyProtection {
  /** The key wrapped by the OS, or undefined where the OS offers no real protection. */
  protectKey(key: Uint8Array): Promise<Uint8Array | undefined>;
  unprotectKey(sealed: Uint8Array): Promise<Uint8Array>;
}

declare global {
  // The desktop preload adds this to window; browsers have none.
  var nexusDesktop: Partial<DesktopKeyProtection> | undefined;
}

const importKey = (raw: Uint8Array): Promise<CryptoKey> =>
  crypto.subtle.importKey("raw", new Uint8Array(raw), "AES-GCM", false, ["encrypt", "decrypt"]);

/**
 * Loads or creates the data key; exported for tests. A key wrapped by the
 * OS is preferred; a non-extractable key made earlier (or where the OS
 * declines) is kept, so a database is never orphaned by a changed choice.
 */
export async function loadDataKey(
  store: KeyValueStore,
  desktop: Partial<DesktopKeyProtection> | undefined,
): Promise<CryptoKey> {
  const wrapped = await store.get(protectedKeyName);
  if (wrapped instanceof Uint8Array && desktop?.unprotectKey !== undefined) {
    return importKey(await desktop.unprotectKey(wrapped));
  }
  const existing = await store.get(keyName);
  if (existing instanceof CryptoKey) {
    return existing;
  }
  if (desktop?.protectKey !== undefined) {
    const raw = crypto.getRandomValues(new Uint8Array(32));
    const sealed = await desktop.protectKey(raw);
    if (sealed !== undefined) {
      await store.put(protectedKeyName, sealed);
      return importKey(raw);
    }
  }
  const created = await crypto.subtle.generateKey({ name: "AES-GCM", length: 256 }, false, [
    "encrypt",
    "decrypt",
  ]);
  await store.put(keyName, created);
  return created;
}

/** Sealed records on top of `store`; exported for tests. */
export function createSealedDatabase(store: KeyValueStore, dataKey: () => Promise<CryptoKey>): LocalDatabase {
  const place = (collection: string, id: string): string => `${collection}${separator}${id}`;
  const bound = (collection: string, id: string): Uint8Array<ArrayBuffer> =>
    new TextEncoder().encode(place(collection, id));
  return {
    get: async (collection, id) => {
      const sealed = await store.get(place(collection, id));
      if (!(sealed instanceof Uint8Array)) {
        return undefined;
      }
      const plaintext = await crypto.subtle.decrypt(
        { name: "AES-GCM", iv: sealed.slice(0, ivLength), additionalData: bound(collection, id) },
        await dataKey(),
        sealed.slice(ivLength),
      );
      return new Uint8Array(plaintext);
    },
    put: async (collection, id, data) => {
      const iv = crypto.getRandomValues(new Uint8Array(ivLength));
      const ciphertext = await crypto.subtle.encrypt(
        { name: "AES-GCM", iv, additionalData: bound(collection, id) },
        await dataKey(),
        new Uint8Array(data),
      );
      const sealed = new Uint8Array(ivLength + ciphertext.byteLength);
      sealed.set(iv);
      sealed.set(new Uint8Array(ciphertext), ivLength);
      await store.put(place(collection, id), sealed);
    },
    delete: (collection, id) => store.delete(place(collection, id)),
    list: async (collection) => {
      const prefix = `${collection}${separator}`;
      const keys = await store.keys(prefix, `${prefix}￿`);
      return keys.map((key) => key.slice(prefix.length));
    },
  };
}

/** Opens the device's database, or a memory one where browsers offer no WebCrypto (plain HTTP). */
export function openDatabase(): Promise<LocalDatabase> {
  if (
    typeof indexedDB === "undefined" ||
    typeof crypto === "undefined" ||
    typeof crypto.subtle === "undefined"
  ) {
    return Promise.resolve(createMemoryDatabase());
  }
  const store = openStore(indexedDB, { databaseName: "nexus-chat-data", storeName: "records" });
  let key: Promise<CryptoKey> | undefined;
  return Promise.resolve(
    createSealedDatabase(store, () => {
      key ??= loadDataKey(store, globalThis.nexusDesktop);
      return key;
    }),
  );
}
