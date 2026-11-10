/**
 * A minimal promise wrapper around one IndexedDB object store, enough for
 * the encrypted storage on web.
 */

/** Get, put and delete by key, and list keys in a range. */
export interface KeyValueStore {
  get(key: string): Promise<unknown>;
  put(key: string, value: unknown): Promise<void>;
  delete(key: string): Promise<void>;
  /** Keys from `lower` to `upper`, both included, in order. */
  keys(lower: string, upper: string): Promise<string[]>;
}

/** Which IndexedDB database and object store to open. */
export interface StoreLocation {
  databaseName: string;
  storeName: string;
}

/** The small secrets (session token, instance address). */
export const secretsLocation: StoreLocation = { databaseName: "nexus-chat", storeName: "secrets" };

/** Waits for an IndexedDB request. */
function completed<T>(request: IDBRequest<T>, failure: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    request.onsuccess = () => {
      resolve(request.result);
    };
    request.onerror = () => {
      reject(request.error ?? new Error(failure));
    };
  });
}

/** Opens the database, creating its one object store on first use. */
function openDatabaseIn(factory: IDBFactory, location: StoreLocation): Promise<IDBDatabase> {
  const request = factory.open(location.databaseName, 1);
  request.onupgradeneeded = () => {
    request.result.createObjectStore(location.storeName);
  };
  return completed(request, "could not open IndexedDB");
}

/** Opens (creating if needed) one of the app's IndexedDB stores. */
export function openStore(
  factory: IDBFactory = indexedDB,
  location: StoreLocation = secretsLocation,
): KeyValueStore {
  const database = openDatabaseIn(factory, location);
  const run = async <T>(
    mode: IDBTransactionMode,
    action: (store: IDBObjectStore) => IDBRequest<T>,
  ): Promise<T> => {
    const store = (await database).transaction(location.storeName, mode).objectStore(location.storeName);
    return completed(action(store), "IndexedDB request failed");
  };
  return {
    get: (key) => run("readonly", (store) => store.get(key)),
    put: async (key, value) => {
      await run("readwrite", (store) => store.put(value, key));
    },
    delete: async (key) => {
      await run("readwrite", (store) => store.delete(key));
    },
    keys: async (lower, upper) => {
      const keys = await run("readonly", (store) => store.getAllKeys(IDBKeyRange.bound(lower, upper)));
      return keys.map(String);
    },
  };
}
