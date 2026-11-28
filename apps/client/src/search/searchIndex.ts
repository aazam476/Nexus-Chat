/**
 * The on-device message search index (spec §4.6). Message content is
 * end-to-end encrypted, so only devices can index it. The index lives in
 * the device's encrypted database, in chunks by conversation (or channel)
 * and month: a search reads only the chunks its filters allow, and the
 * same chunks are what a device can later share with the user's other
 * devices, encrypted, instead of each rebuilding them.
 *
 * Each chunk holds its messages (sender, time, whether they have
 * attachments, and the text, for showing results and for undoing an edit's
 * terms) and, for each term, the messages that contain it.
 */
import type { LocalDatabase } from "../storage/localDatabase";
import { terms } from "./tokens";

/** One indexed message. */
export interface IndexedDoc {
  conversationId: string;
  /** `@name@instance`, the sender's user. */
  sender: string;
  sentAt: string;
  text: string;
  hasAttachments: boolean;
  /** For a reply in a thread, the message the thread is on. */
  thread?: string;
}

/**
 * A chunk of the index: one conversation's messages from one month. Maps,
 * not objects, since terms come from messages and could be any word,
 * `__proto__` included.
 */
export interface Chunk {
  docs: Map<string, IndexedDoc>;
  /** Each term, with the IDs of the messages that contain it. */
  terms: Map<string, string[]>;
}

/** How a chunk is stored: its maps as lists of pairs. */
interface StoredChunk {
  docs: [string, IndexedDoc][];
  terms: [string, string[]][];
}

/** Collections in the device's database. */
const collections = {
  chunks: "search-chunks",
  locations: "search-locations",
  /** Chunks changed here since they were last shared with the user's other devices. */
  changed: "search-changed",
} as const;

/** The chunk a message from `sentAt` belongs in. */
export function chunkKey(conversationId: string, sentAt: string): string {
  return `${conversationId}|${sentAt.slice(0, 7)}`;
}

const encoder = new TextEncoder();
const decoder = new TextDecoder();

/** Reads a chunk; a missing one is empty. */
export async function readChunk(database: LocalDatabase, key: string): Promise<Chunk> {
  const stored = await database.get(collections.chunks, key);
  return stored === undefined ? { docs: new Map(), terms: new Map() } : decodeChunk(stored);
}

/** A chunk as bytes, to store or to share. */
export function encodeChunk(chunk: Chunk): Uint8Array {
  const stored: StoredChunk = { docs: [...chunk.docs], terms: [...chunk.terms] };
  return encoder.encode(JSON.stringify(stored));
}

/** A chunk from bytes. */
export function decodeChunk(bytes: Uint8Array): Chunk {
  const parsed = JSON.parse(decoder.decode(bytes)) as StoredChunk;
  return { docs: new Map(parsed.docs), terms: new Map(parsed.terms) };
}

/**
 * Saves a chunk, or forgets it once it is empty, and notes it changed so
 * it is shared with the user's other devices; `shared` is for a chunk that
 * came from them, which needs no sharing back.
 */
export async function writeChunk(
  database: LocalDatabase,
  key: string,
  chunk: Chunk,
  shared = false,
): Promise<void> {
  if (chunk.docs.size === 0) {
    await database.delete(collections.chunks, key);
  } else {
    await database.put(collections.chunks, key, encodeChunk(chunk));
  }
  if (!shared) {
    await database.put(collections.changed, key, new Uint8Array([1]));
  }
}

/** The chunks changed here since they were last shared. */
export function changedChunks(database: LocalDatabase): Promise<string[]> {
  return database.list(collections.changed);
}

/** Notes that a chunk was shared as it now is. */
export function markShared(database: LocalDatabase, key: string): Promise<void> {
  return database.delete(collections.changed, key);
}

/** Every chunk key on the device, sorted. */
export function chunkKeys(database: LocalDatabase): Promise<string[]> {
  return database.list(collections.chunks);
}

/** The chunk a message's entry is in, so an edit or deletion can find it. */
export async function locationOf(database: LocalDatabase, messageId: string): Promise<string | undefined> {
  const stored = await database.get(collections.locations, messageId);
  return stored === undefined ? undefined : decoder.decode(stored);
}

/** Notes (or with undefined, forgets) where a message's entry is. */
export async function setLocation(
  database: LocalDatabase,
  messageId: string,
  key: string | undefined,
): Promise<void> {
  if (key === undefined) {
    await database.delete(collections.locations, messageId);
  } else {
    await database.put(collections.locations, messageId, encoder.encode(key));
  }
}

/** Adds a message to a chunk, or replaces its entry. */
export function addDoc(chunk: Chunk, id: string, doc: IndexedDoc): void {
  removeDoc(chunk, id);
  chunk.docs.set(id, doc);
  for (const term of new Set(terms(doc.text))) {
    chunk.terms.set(term, [...(chunk.terms.get(term) ?? []), id]);
  }
}

/** Takes a message out of a chunk, with its terms. */
export function removeDoc(chunk: Chunk, id: string): void {
  const doc = chunk.docs.get(id);
  if (doc === undefined) {
    return;
  }
  for (const term of new Set(terms(doc.text))) {
    const remaining = (chunk.terms.get(term) ?? []).filter((other) => other !== id);
    if (remaining.length === 0) {
      chunk.terms.delete(term);
    } else {
      chunk.terms.set(term, remaining);
    }
  }
  chunk.docs.delete(id);
}
