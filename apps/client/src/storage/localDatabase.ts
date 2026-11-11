/**
 * The device's encrypted database (spec §5.3, ADR 0013): MLS group state,
 * history keys, messages and the search index, stored as opaque records by
 * collection and ID. Each platform has its own implementation file
 * (`database.native.ts`: SQLCipher; `database.web.ts`: sealed IndexedDB
 * records), chosen by the bundler; this file holds the shared contract and
 * the in-memory version used where neither applies.
 */

/** Records by collection ("groups", "history-keys", …) and ID. */
export interface LocalDatabase {
  get(collection: string, id: string): Promise<Uint8Array | undefined>;
  put(collection: string, id: string, data: Uint8Array): Promise<void>;
  delete(collection: string, id: string): Promise<void>;
  /** The IDs in a collection, sorted. */
  list(collection: string): Promise<string[]>;
}

/** Keeps records in memory; nothing survives a restart. */
export function createMemoryDatabase(): LocalDatabase {
  const records = new Map<string, Map<string, Uint8Array>>();
  const collectionOf = (name: string): Map<string, Uint8Array> => {
    let collection = records.get(name);
    if (collection === undefined) {
      collection = new Map();
      records.set(name, collection);
    }
    return collection;
  };
  return {
    get: (collection, id) => Promise.resolve(collectionOf(collection).get(id)),
    put: (collection, id, data) => {
      collectionOf(collection).set(id, data.slice());
      return Promise.resolve();
    },
    delete: (collection, id) => {
      collectionOf(collection).delete(id);
      return Promise.resolve();
    },
    list: (collection) => Promise.resolve([...collectionOf(collection).keys()].sort()),
  };
}

/**
 * One account's part of the device's database: every collection name is
 * prefixed with `scope`, so two accounts signed in on the same device in
 * turn never read each other's keys or messages.
 */
export function scopedDatabase(database: LocalDatabase, scope: string): LocalDatabase {
  const scoped = (collection: string): string => `${scope}/${collection}`;
  return {
    get: (collection, id) => database.get(scoped(collection), id),
    put: (collection, id, data) => database.put(scoped(collection), id, data),
    delete: (collection, id) => database.delete(scoped(collection), id),
    list: (collection) => database.list(scoped(collection)),
  };
}
