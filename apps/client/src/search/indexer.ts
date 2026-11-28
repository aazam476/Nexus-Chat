/**
 * Keeps the search index in step with the messages a device stores
 * (spec §4.6): new messages are added, edits replace their text and
 * deletions take them out, by the same rule the conversation view uses
 * (only the sender's user may edit or delete). Reactions are not
 * searchable. Messages older than the recent window are left out unless
 * the user asked to search older messages for that conversation.
 */
import { userOf } from "@nexus-chat/crypto";

import type { DecryptedMessage } from "../messaging/engine/sync";
import type { LocalDatabase } from "../storage/localDatabase";
import {
  addDoc,
  chunkKey,
  locationOf,
  readChunk,
  removeDoc,
  setLocation,
  writeChunk,
  type Chunk,
} from "./searchIndex";

/** How far back a device indexes by default: 90 days (spec §4.6). */
export const recentWindowDays = 90;

/** Conversations whose whole history the user asked to search. */
const everythingFlags = "search-everything";

/** From now on, index the conversation's messages of any age. */
export async function indexEverything(database: LocalDatabase, conversationId: string): Promise<void> {
  await database.put(everythingFlags, conversationId, new Uint8Array([1]));
}

/** Whether the conversation's whole history is being indexed. */
export async function indexesEverything(database: LocalDatabase, conversationId: string): Promise<boolean> {
  return (await database.get(everythingFlags, conversationId)) !== undefined;
}

/** Chunks read and changed during one pass, written back at its end. */
class Pass {
  private readonly chunks = new Map<string, Chunk>();

  constructor(private readonly database: LocalDatabase) {}

  async chunk(key: string): Promise<Chunk> {
    let chunk = this.chunks.get(key);
    if (chunk === undefined) {
      chunk = await readChunk(this.database, key);
      this.chunks.set(key, chunk);
    }
    return chunk;
  }

  async finish(): Promise<void> {
    for (const [key, chunk] of this.chunks) {
      await writeChunk(this.database, key, chunk);
    }
  }
}

/** What indexing needs to know besides the messages. */
export interface IndexOptions {
  /** Index messages of any age, as "search older messages" asks. */
  everything?: boolean;
  /** The time to measure the recent window from; now if missing. */
  now?: Date;
}

function senderOf(message: DecryptedMessage): string {
  return userOf(message.sender) ?? message.sender;
}

function isRecent(sentAt: string, options: IndexOptions): boolean {
  const now = (options.now ?? new Date()).getTime();
  return options.everything === true || now - Date.parse(sentAt) <= recentWindowDays * 86_400_000;
}

/** Adds a new message to its chunk. */
async function addMessage(
  pass: Pass,
  database: LocalDatabase,
  conversationId: string,
  message: DecryptedMessage,
): Promise<void> {
  if (message.payload.kind !== "text") {
    return;
  }
  const key = chunkKey(conversationId, message.sentAt);
  addDoc(await pass.chunk(key), message.id, {
    conversationId,
    sender: senderOf(message),
    sentAt: message.sentAt,
    text: message.payload.text,
    hasAttachments: (message.payload.attachments?.length ?? 0) > 0,
    ...(message.thread === undefined ? {} : { thread: message.thread }),
  });
  await setLocation(database, message.id, key);
}

/** Applies an edit or deletion to the message it names, if that message is indexed and the sender's. */
async function changeMessage(pass: Pass, database: LocalDatabase, message: DecryptedMessage): Promise<void> {
  const { payload } = message;
  if (payload.kind !== "edit" && payload.kind !== "delete") {
    return;
  }
  const key = await locationOf(database, payload.target);
  const chunk = key === undefined ? undefined : await pass.chunk(key);
  const doc = chunk?.docs.get(payload.target);
  if (chunk === undefined || doc?.sender !== senderOf(message)) {
    return;
  }
  if (payload.kind === "edit") {
    addDoc(chunk, payload.target, { ...doc, text: payload.text });
  } else {
    removeDoc(chunk, payload.target);
    await setLocation(database, payload.target, undefined);
  }
}

/** Brings the index up to date with messages a conversation just stored, oldest first. */
export async function indexMessages(
  database: LocalDatabase,
  conversationId: string,
  messages: readonly DecryptedMessage[],
  options: IndexOptions = {},
): Promise<void> {
  const pass = new Pass(database);
  const everything = options.everything === true || (await indexesEverything(database, conversationId));
  const wanted = { ...options, everything };
  const ordered = [...messages].sort((first, second) => first.sentAt.localeCompare(second.sentAt));
  for (const message of ordered) {
    if (message.payload.kind === "text" && isRecent(message.sentAt, wanted)) {
      await addMessage(pass, database, conversationId, message);
    } else {
      await changeMessage(pass, database, message);
    }
  }
  await pass.finish();
}

/** Takes messages out of the index, as a moderator's deletion asks. */
export async function unindexMessages(database: LocalDatabase, messageIds: readonly string[]): Promise<void> {
  const pass = new Pass(database);
  for (const id of messageIds) {
    const key = await locationOf(database, id);
    if (key !== undefined) {
      removeDoc(await pass.chunk(key), id);
      await setLocation(database, id, undefined);
    }
  }
  await pass.finish();
}
