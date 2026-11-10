/**
 * iOS and Android: an SQLite database encrypted with SQLCipher through
 * op-sqlite (ADR 0013). Its key is 32 random bytes kept in the Keychain or
 * Keystore (expo-secure-store), readable only by this app on this device,
 * so a copied database file is unreadable.
 */
import { open, type DB } from "@op-engineering/op-sqlite";
import { getRandomBytes } from "expo-crypto";
import * as SecureStore from "expo-secure-store";

import type { LocalDatabase } from "./localDatabase";

const keyName = "database-key";
const fileName = "nexus-chat.db";

async function databaseKey(): Promise<string> {
  const existing = await SecureStore.getItemAsync(keyName);
  if (existing !== null) {
    return existing;
  }
  const created = Array.from(getRandomBytes(32), (byte) => byte.toString(16).padStart(2, "0")).join("");
  await SecureStore.setItemAsync(keyName, created, {
    keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY,
  });
  return created;
}

/** Records on top of an open database; exported for tests. */
export async function createSqliteDatabase(database: DB): Promise<LocalDatabase> {
  await database.execute(
    "CREATE TABLE IF NOT EXISTS records (collection TEXT NOT NULL, id TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY (collection, id))",
  );
  return {
    get: async (collection, id) => {
      const result = await database.execute("SELECT data FROM records WHERE collection = ? AND id = ?", [
        collection,
        id,
      ]);
      const data = result.rows[0]?.data;
      return data instanceof ArrayBuffer ? new Uint8Array(data) : undefined;
    },
    put: async (collection, id, data) => {
      await database.execute("INSERT OR REPLACE INTO records (collection, id, data) VALUES (?, ?, ?)", [
        collection,
        id,
        data.slice().buffer,
      ]);
    },
    delete: async (collection, id) => {
      await database.execute("DELETE FROM records WHERE collection = ? AND id = ?", [collection, id]);
    },
    list: async (collection) => {
      const result = await database.execute("SELECT id FROM records WHERE collection = ? ORDER BY id", [
        collection,
      ]);
      return result.rows.flatMap((row) => (typeof row.id === "string" ? [row.id] : []));
    },
  };
}

/** Opens (creating if needed) the device's encrypted database. */
export async function openDatabase(): Promise<LocalDatabase> {
  return createSqliteDatabase(open({ name: fileName, encryptionKey: await databaseKey() }));
}
