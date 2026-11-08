/**
 * A minimal promise wrapper around one IndexedDB object store, enough for
 * the encrypted storage on web.
 */

/** Get, put and delete by key. */
export interface KeyValueStore {
  get(key: string): Promise<unknown>;
  put(key: string, value: unknown): Promise<void>;
  delete(key: string): Promise<void>;
}

const databaseName = "nexus-chat";
const storeName = "secrets";

/** Opens (creating if needed) the app's IndexedDB store. */
export function openStore(factory: IDBFactory = indexedDB): KeyValueStore {
  const database = new Promise<IDBDatabase>((resolve, reject) => {
    const request = factory.open(databaseName, 1);
    request.onupgradeneeded = () => {
      request.result.createObjectStore(storeName);
    };
    request.onsuccess = () => {
      resolve(request.result);
    };
    request.onerror = () => {
      reject(request.error ?? new Error("could not open IndexedDB"));
    };
  });
  const run = async <T>(
    mode: IDBTransactionMode,
    action: (store: IDBObjectStore) => IDBRequest<T>,
  ): Promise<T> => {
    const store = (await database).transaction(storeName, mode).objectStore(storeName);
    return new Promise<T>((resolve, reject) => {
      const request = action(store);
      request.onsuccess = () => {
        resolve(request.result);
      };
      request.onerror = () => {
        reject(request.error ?? new Error("IndexedDB request failed"));
      };
    });
  };
  return {
    get: (key) => run("readonly", (store) => store.get(key)),
    put: async (key, value) => {
      await run("readwrite", (store) => store.put(value, key));
    },
    delete: async (key) => {
      await run("readwrite", (store) => store.delete(key));
    },
  };
}
