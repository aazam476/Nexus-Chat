/**
 * History on a user's other devices (ADR 0003, docs/e2ee-design.md §4–5).
 *
 * Every message also goes to the server as an archive copy, sealed under
 * the conversation's history key for its epoch. Each device keeps the
 * history key of every epoch it passes through and adds it to the user's
 * key backup, one sealed blob per conversation. A device that restores
 * from the backup later reads the archive copies it missed with those
 * keys, checking each sender as for live messages.
 */
import {
  decodeDeviceCredential,
  epoch,
  historyKey,
  openArchive,
  openBlob,
  sealBlob,
  type Group,
} from "@nexus-chat/crypto";

import { decodePayload } from "../payload";
import type { StoredArchive } from "./api";
import { fromBase64, toBase64, utf8 } from "./bytes";
import type { EngineContext } from "./context";
import { loadMessages, storeMessages } from "./history";
import type { DecryptedMessage } from "./sync";

/** History keys by epoch, base64. */
type HistoryKeys = Record<string, string>;

const blobOf = (conversationId: string): string => `history.${conversationId}`;
const local = { collection: "history-keys", read: "history-read" } as const;
/** How many pages of the archive a device reads back at most when it joins. */
const maximumPages = 50;

const parse = (bytes: Uint8Array): HistoryKeys => JSON.parse(new TextDecoder().decode(bytes)) as HistoryKeys;

/** The history keys this device keeps for a conversation or channel. */
export async function keptHistoryKeys(context: EngineContext, conversationId: string): Promise<HistoryKeys> {
  const stored = await context.database.get(local.collection, conversationId);
  return stored === undefined ? {} : parse(stored);
}

/** The keys in the backup; none if there is no blob or it cannot be opened. */
async function backedUpKeys(context: EngineContext, conversationId: string): Promise<HistoryKeys> {
  const name = blobOf(conversationId);
  const sealed = await context.api.getBlob(name);
  if (sealed === undefined) {
    return {};
  }
  try {
    return parse(await openBlob(context.backupKey, name, sealed));
  } catch {
    return {};
  }
}

/** Adds the group's current history key to what this device keeps, if new. */
export async function noteHistoryKey(
  conversationId: string,
  group: Group,
  noted: HistoryKeys,
): Promise<void> {
  noted[String(epoch(group))] ??= toBase64(await historyKey(group, utf8(conversationId)));
}

/**
 * Keeps `noted` keys on the device and, when any is new, merges them into
 * the key backup, so the user's other devices can read this history.
 */
export async function saveHistoryKeys(
  context: EngineContext,
  conversationId: string,
  noted: HistoryKeys,
): Promise<void> {
  const kept = await keptHistoryKeys(context, conversationId);
  if (Object.keys(noted).every((at) => kept[at] !== undefined)) {
    return;
  }
  const merged = { ...(await backedUpKeys(context, conversationId)), ...kept, ...noted };
  const encoded = utf8(JSON.stringify(merged));
  const name = blobOf(conversationId);
  await context.api.putBlob(name, await sealBlob(context.backupKey, name, encoded));
  await context.database.put(local.collection, conversationId, encoded);
}

/**
 * Opens one archive copy, or undefined when it has no key or fails a check.
 * A server channel's copies are addressed by the channel's ID.
 */
export async function openCopy(
  context: EngineContext,
  conversationId: string,
  copy: StoredArchive,
  key: string | undefined,
): Promise<DecryptedMessage | undefined> {
  if (key === undefined) {
    return undefined;
  }
  try {
    const opened = await openArchive(
      { epoch: BigInt(copy.epoch), sealed: copy.sealed },
      { channelId: utf8(conversationId), messageId: utf8(copy.messageId) },
      fromBase64(key),
      context.check,
    );
    const envelope = decodePayload(opened.plaintext);
    const sender = decodeDeviceCredential(opened.sender)?.identity;
    return envelope?.id === copy.messageId && sender !== undefined
      ? { id: envelope.id, sender, payload: envelope.payload, sentAt: copy.createdAt }
      : undefined;
  } catch {
    return undefined;
  }
}

/** Every archive copy this device can open, newest first. */
async function readArchive(
  context: EngineContext,
  conversationId: string,
  keys: HistoryKeys,
): Promise<DecryptedMessage[]> {
  const opened: DecryptedMessage[] = [];
  let before: string | undefined;
  for (let page = 0; page < maximumPages; page++) {
    const copies = await context.api.archive(conversationId, before);
    for (const copy of copies) {
      const message = await openCopy(context, conversationId, copy, keys[String(copy.epoch)]);
      if (message !== undefined) {
        opened.push(message);
      }
    }
    before = copies.at(-1)?.createdAt;
    if (before === undefined) {
      break;
    }
  }
  return opened;
}

/**
 * The first time this device is in a conversation, reads the history it
 * missed from the archive with the backed-up keys. Later opens do nothing.
 */
export async function readMissedHistory(context: EngineContext, conversationId: string): Promise<void> {
  if ((await context.database.get(local.read, conversationId)) !== undefined) {
    return;
  }
  const keys = {
    ...(await backedUpKeys(context, conversationId)),
    ...(await keptHistoryKeys(context, conversationId)),
  };
  if (Object.keys(keys).length > 0) {
    const held = new Set((await loadMessages(context, conversationId)).map((message) => message.id));
    const missed = await readArchive(context, conversationId, keys);
    await storeMessages(
      context,
      conversationId,
      missed.filter((message) => !held.has(message.id)),
    );
  }
  await context.database.put(local.read, conversationId, utf8("1"));
}
