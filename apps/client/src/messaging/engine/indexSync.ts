/**
 * Sharing the search index between the user's own devices (spec §4.6):
 * a device uploads its index chunks (one per conversation or channel and
 * month) to the key backup, each sealed under the backup key with its name
 * bound in, and a new device downloads the recent chunks it lacks instead
 * of rebuilding them. The server sees only opaque blobs, their names and
 * their sizes.
 */
import { openBlob, sealBlob } from "@nexus-chat/crypto";

import { recentWindowDays } from "../../search/indexer";
import {
  addDoc,
  changedChunks,
  decodeChunk,
  encodeChunk,
  markShared,
  readChunk,
  setLocation,
  writeChunk,
} from "../../search/searchIndex";
import type { EngineContext } from "./context";

/** Blob names: lowercase, so IDs (UUIDs) and months fit as they are. */
const prefix = "search.";
/** The largest chunk shared; bigger ones are rebuilt on each device instead. */
const maximumChunkBytes = 900 * 1024;
/** The most chunks kept in the backup, below its 10,000-blob limit for keys and settings. */
const maximumChunks = 5000;

function blobName(key: string): string {
  return prefix + key.replace("|", ".");
}

function keyOf(name: string): string {
  const [conversationId, month] = name.slice(prefix.length).split(/\.(?=[^.]+$)/);
  return `${conversationId ?? ""}|${month ?? ""}`;
}

/** Uploads the chunks changed here, and returns how many went up. */
export async function shareIndex(context: EngineContext): Promise<number> {
  const { database } = context;
  const existing = new Set((await context.api.getBackup()).blobs.filter((name) => name.startsWith(prefix)));
  let shared = 0;
  for (const key of await changedChunks(database)) {
    const name = blobName(key);
    const bytes = encodeChunk(await readChunk(database, key));
    if (bytes.length <= maximumChunkBytes && (existing.has(name) || existing.size < maximumChunks)) {
      await context.api.putBlob(name, await sealBlob(context.backupKey, name, bytes));
      existing.add(name);
      shared += 1;
    }
    await markShared(database, key);
  }
  return shared;
}

/** The first month of the recent window, as chunk keys write it. */
function firstRecentMonth(now: Date): string {
  return new Date(now.getTime() - recentWindowDays * 86_400_000).toISOString().slice(0, 7);
}

/**
 * Downloads the user's shared chunks from the recent window (or all of
 * them, with `everything`) and adds what they hold to this device's index.
 * It returns how many chunks it read.
 */
export async function fetchIndex(
  context: EngineContext,
  options: { everything?: boolean; now?: Date } = {},
): Promise<number> {
  const { database } = context;
  const since = options.everything === true ? "" : firstRecentMonth(options.now ?? new Date());
  const names = (await context.api.getBackup()).blobs.filter(
    (name) => name.startsWith(prefix) && (keyOf(name).split("|")[1] ?? "") >= since,
  );
  let read = 0;
  for (const name of names) {
    const sealed = await context.api.getBlob(name);
    if (sealed === undefined) {
      continue;
    }
    const key = keyOf(name);
    const local = await readChunk(database, key);
    for (const [id, doc] of decodeChunk(await openBlob(context.backupKey, name, sealed)).docs) {
      if (!local.docs.has(id)) {
        addDoc(local, id, doc);
        await setLocation(database, id, key);
      }
    }
    await writeChunk(database, key, local, true);
    read += 1;
  }
  return read;
}
