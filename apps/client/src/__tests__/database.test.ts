import "fake-indexeddb/auto";
import { open, type DB, type QueryResult, type Scalar } from "@op-engineering/op-sqlite";

import { createMemoryDatabase, scopedDatabase, type LocalDatabase } from "../storage/localDatabase";
import { createSqliteDatabase, openDatabase as openNative } from "../storage/database.native";
import { createSealedDatabase, loadDataKey, openDatabase as openWeb } from "../storage/database.web";
import { openStore, type KeyValueStore } from "../storage/indexedDb";

jest.mock("expo-secure-store", () => {
  const values = new Map<string, string>();
  return {
    WHEN_UNLOCKED_THIS_DEVICE_ONLY: 1,
    getItemAsync: jest.fn((key: string) => Promise.resolve(values.get(key) ?? null)),
    setItemAsync: jest.fn((key: string, value: string) => {
      values.set(key, value);
      return Promise.resolve();
    }),
  };
});
jest.mock("expo-crypto", () => ({ getRandomBytes: (length: number) => new Uint8Array(length).fill(7) }));
jest.mock("@op-engineering/op-sqlite", () => ({ open: jest.fn() }));

const bytes = (...values: number[]): Uint8Array => new Uint8Array(values);

/** The contract every implementation keeps. */
async function behavesAsADatabase(database: LocalDatabase): Promise<void> {
  expect(await database.get("groups", "a")).toBeUndefined();
  await database.put("groups", "b", bytes(2));
  await database.put("groups", "a", bytes(1));
  await database.put("history-keys", "a", bytes(9));
  expect(await database.get("groups", "a")).toEqual(bytes(1));
  expect(await database.list("groups")).toEqual(["a", "b"]);
  await database.put("groups", "a", bytes(3));
  expect(await database.get("groups", "a")).toEqual(bytes(3));
  await database.delete("groups", "a");
  expect(await database.list("groups")).toEqual(["b"]);
  expect(await database.list("empty")).toEqual([]);
}

/** A tiny stand-in for op-sqlite that understands the four statements the database uses. */
function fakeSqlite(): DB {
  const rows = new Map<string, { collection: string; id: string; data: ArrayBuffer }>();
  const result = (found: Record<string, Scalar>[]): QueryResult => ({ rowsAffected: 0, rows: found });
  const execute = (query: string, params: Scalar[] = []): Promise<QueryResult> => {
    const [collection, id, data] = params as [string, string, ArrayBuffer];
    if (query.startsWith("INSERT")) {
      rows.set(`${collection}/${id}`, { collection, id, data });
    } else if (query.startsWith("DELETE")) {
      rows.delete(`${collection}/${id}`);
    } else if (query.startsWith("SELECT data")) {
      const row = rows.get(`${collection}/${id}`);
      return Promise.resolve(result(row === undefined ? [] : [{ data: row.data }]));
    } else if (query.startsWith("SELECT id")) {
      const ids = [...rows.values()].filter((row) => row.collection === collection).map((row) => row.id);
      return Promise.resolve(result([...ids.sort().map((value) => ({ id: value })), { id: 5 }]));
    }
    return Promise.resolve(result([]));
  };
  return { execute } as unknown as DB;
}

describe("the local database", () => {
  it("keeps records in memory where no platform file applies", async () => {
    await behavesAsADatabase(createMemoryDatabase());
  });

  it("keeps each account's records apart", async () => {
    const shared = createMemoryDatabase();
    const ada = scopedDatabase(shared, "ada@example.com");
    const grace = scopedDatabase(shared, "grace@example.com");
    await behavesAsADatabase(ada);
    await ada.put("keys", "identity", new Uint8Array([1]));
    expect(await grace.get("keys", "identity")).toBeUndefined();
    expect(await grace.list("keys")).toEqual([]);
    await grace.put("keys", "identity", new Uint8Array([2]));
    expect(await ada.get("keys", "identity")).toEqual(new Uint8Array([1]));
  });

  it("stores records in SQLCipher on native, keyed from secure storage", async () => {
    jest.mocked(open).mockReturnValue(fakeSqlite());
    await behavesAsADatabase(await openNative());
    await openNative();

    // The same 32-byte key both times, made once.
    const calls = jest.mocked(open).mock.calls;
    expect(calls[0]?.[0]).toEqual({ name: "nexus-chat.db", encryptionKey: "07".repeat(32) });
    expect(calls[1]?.[0]).toEqual(calls[0]?.[0]);
    await behavesAsADatabase(await createSqliteDatabase(fakeSqlite()));
  });

  it("seals each record in IndexedDB on web, bound to its place", async () => {
    const store = openStore(indexedDB, { databaseName: "test-sealed", storeName: "records" });
    const key = loadDataKey(store, undefined);
    const database = createSealedDatabase(store, () => key);
    await behavesAsADatabase(database);

    // Stored bytes are not the plaintext, and moving them elsewhere fails.
    await database.put("groups", "x", bytes(1, 2, 3));
    const sealed = await store.get("groups\u0001x");
    expect(sealed).not.toEqual(bytes(1, 2, 3));
    await store.put("groups\u0001y", sealed);
    await expect(database.get("groups", "y")).rejects.toThrow();
    await behavesAsADatabase(await openWeb());
  });

  it("keeps the same data key across launches", async () => {
    // Browsers keep CryptoKey objects in IndexedDB; fake-indexeddb can't, so a Map stands in.
    const store = mapStore();
    const first = await loadDataKey(store, undefined);
    const iv = new Uint8Array(12);
    const sealed = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, first, new Uint8Array([1]));
    const second = await loadDataKey(store, undefined);

    expect(new Uint8Array(await crypto.subtle.decrypt({ name: "AES-GCM", iv }, second, sealed))).toEqual(
      bytes(1),
    );
    expect(first.extractable).toBe(false);
  });

  it("has the desktop shell wrap the data key with the operating system", async () => {
    const store = openStore(indexedDB, { databaseName: "test-desktop", storeName: "records" });
    const protect = jest.fn((key: Uint8Array) => Promise.resolve(key.map((byte) => byte ^ 0xff)));
    const desktop = { protectKey: protect, unprotectKey: protect };
    const first = createSealedDatabase(store, () => loadDataKey(store, desktop));
    await first.put("groups", "a", bytes(4));
    const again = createSealedDatabase(store, () => loadDataKey(store, desktop));

    expect(await again.get("groups", "a")).toEqual(bytes(4));
    // Wrapped once when made, unwrapped once on the next launch.
    expect(protect).toHaveBeenCalledTimes(2);
    const wrapped = await store.get("\u0000protected-key");
    expect(wrapped).toBeInstanceOf(Uint8Array);
  });

  it("keeps a non-extractable key where the OS declines to wrap one", async () => {
    const store = mapStore();
    const declines = { protectKey: () => Promise.resolve(undefined), unprotectKey: jest.fn() };
    const first = await loadDataKey(store, declines);

    expect(first.extractable).toBe(false);
    expect(await loadDataKey(store, declines)).toBe(first);
    expect(declines.unprotectKey).not.toHaveBeenCalled();
  });

  it("falls back to memory where browsers offer no WebCrypto", async () => {
    const subtle = crypto.subtle;
    Object.defineProperty(crypto, "subtle", { value: undefined, configurable: true });
    try {
      await behavesAsADatabase(await openWeb());
    } finally {
      Object.defineProperty(crypto, "subtle", { value: subtle, configurable: true });
    }
  });
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
    keys: () => Promise.resolve([...values.keys()]),
  };
}
