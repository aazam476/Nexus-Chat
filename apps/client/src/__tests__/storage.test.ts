import "fake-indexeddb/auto";
import * as SecureStore from "expo-secure-store";

import { openStore, type KeyValueStore } from "../storage/indexedDb";
import { createSecureStorage as createPlatformDefault } from "../storage/platformStorage";
import { createSecureStorage as createNativeStorage } from "../storage/platformStorage.native";
import {
  createEncryptedStorage,
  createSecureStorage as createWebStorage,
} from "../storage/platformStorage.web";
import { createMemoryStorage, type SecureStorage } from "../storage/storage";

jest.mock("expo-secure-store", () => {
  const values = new Map<string, string>();
  return {
    getItemAsync: jest.fn((key: string) => Promise.resolve(values.get(key) ?? null)),
    setItemAsync: jest.fn((key: string, value: string) => {
      values.set(key, value);
      return Promise.resolve();
    }),
    deleteItemAsync: jest.fn((key: string) => {
      values.delete(key);
      return Promise.resolve();
    }),
  };
});

/** A key-value store in memory, which keeps CryptoKey objects as browsers do. */
function mapStore(): KeyValueStore {
  const values = new Map<string, unknown>();
  return {
    get: (key) => Promise.resolve(values.get(key)),
    put: (key, value) => {
      values.set(key, value);
      return Promise.resolve();
    },
    delete: (key) => {
      values.delete(key);
      return Promise.resolve();
    },
  };
}

async function roundTrip(storage: SecureStorage): Promise<void> {
  expect(await storage.get("session")).toBeUndefined();
  await storage.set("session", "nxs_secret");
  expect(await storage.get("session")).toBe("nxs_secret");
  await storage.remove("session");
  expect(await storage.get("session")).toBeUndefined();
}

describe("storage", () => {
  it("keeps values in memory", async () => {
    await roundTrip(createMemoryStorage());
    await roundTrip(createPlatformDefault());
  });

  it("uses the Keychain and Keystore on native", async () => {
    await roundTrip(createNativeStorage());
    expect(SecureStore.setItemAsync).toHaveBeenCalledWith("session", "nxs_secret");
  });

  it("encrypts values on web, with a key that cannot be read out", async () => {
    const store = mapStore();
    const storage = createEncryptedStorage(store);
    await roundTrip(storage);

    await storage.set("session", "nxs_secret");
    const sealed = await store.get("session");
    expect(sealed).toBeInstanceOf(Uint8Array);
    expect(new TextDecoder().decode(sealed as Uint8Array)).not.toContain("nxs_secret");
    expect(((await store.get("storage-key")) as CryptoKey).extractable).toBe(false);

    const reopened = createEncryptedStorage(store);
    expect(await reopened.get("session")).toBe("nxs_secret");
  });

  it("forgets values sealed with a key that is gone", async () => {
    const store = mapStore();
    await createEncryptedStorage(store).set("session", "nxs_secret");
    await store.delete("storage-key");

    expect(await createEncryptedStorage(store).get("session")).toBeUndefined();
    expect(await store.get("session")).toBeUndefined();
  });

  it("keeps values in IndexedDB", async () => {
    const store = openStore(indexedDB);
    await store.put("a", new Uint8Array([1, 2]));
    expect(await store.get("a")).toEqual(new Uint8Array([1, 2]));
    await store.delete("a");
    expect(await store.get("a")).toBeUndefined();
  });

  it("uses IndexedDB in a browser, and memory where there is none", async () => {
    await roundTrip(createWebStorage());
    const saved = globalThis.indexedDB;
    Reflect.deleteProperty(globalThis, "indexedDB");
    await roundTrip(createWebStorage());
    globalThis.indexedDB = saved;
  });

  it("reports IndexedDB failures", async () => {
    const failing = {
      open: () => {
        const request = {
          onupgradeneeded: null,
          onsuccess: null,
          onerror: null as (() => void) | null,
          error: null,
          result: undefined,
        };
        setTimeout(() => request.onerror?.(), 0);
        return request;
      },
    } as unknown as IDBFactory;

    await expect(openStore(failing).get("x")).rejects.toThrow("could not open IndexedDB");
  });
});
