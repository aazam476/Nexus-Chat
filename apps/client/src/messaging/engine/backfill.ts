/**
 * "Search older messages" (spec §4.6): beyond the recent window, a device
 * indexes a conversation's or channel's whole history when asked. It first
 * indexes everything it already holds, then reads the archive further back
 * a page at a time, keeping where it got to after each page, so the work
 * resumes after an interruption and can wait for a good moment.
 */
import { indexEverything, indexMessages } from "../../search/indexer";
import type { StoredArchive } from "./api";
import { backedUpKeys, keptHistoryKeys, openCopy, type HistoryKeys } from "./archive";
import { utf8 } from "./bytes";
import { channelKeysId, keyName, type Audience } from "./channels";
import type { EngineContext } from "./context";
import { loadMessages, storeMessages } from "./history";
import type { DecryptedMessage } from "./sync";

const cursors = "search-backfill";
/** The cursor of a history read to its start. */
const finished = "done";

/** How far a backfill has come. */
export interface BackfillProgress {
  /** Messages added to the index so far in this run. */
  indexed: number;
  /** Whether the whole history is indexed. */
  done: boolean;
}

/** Where a conversation's archive is read from: a DM or group DM, or a channel of an audience. */
export type HistorySource = { conversationId: string } | { channelId: string; audience: Audience };

function idOf(source: HistorySource): string {
  return "conversationId" in source ? source.conversationId : source.channelId;
}

/** A page of archive copies before `before`, newest first. */
async function page(
  context: EngineContext,
  source: HistorySource,
  before: string | undefined,
): Promise<(StoredArchive & { groupId?: Uint8Array })[]> {
  if ("conversationId" in source) {
    return context.api.archive(source.conversationId, before);
  }
  const channel = { serverId: source.audience.serverId, channelId: source.channelId };
  return context.api.channelArchive(channel, before);
}

/** The history keys that open the source's copies, by the names its copies use. */
async function keysFor(context: EngineContext, source: HistorySource): Promise<HistoryKeys> {
  if ("conversationId" in source) {
    return {
      ...(await backedUpKeys(context, source.conversationId)),
      ...(await keptHistoryKeys(context, source.conversationId)),
    };
  }
  return keptHistoryKeys(context, channelKeysId(source.channelId));
}

/** Opens a page's copies this device does not hold yet. */
async function openPage(
  context: EngineContext,
  source: HistorySource,
  copies: Awaited<ReturnType<typeof page>>,
  held: Set<string>,
): Promise<DecryptedMessage[]> {
  const keys = await keysFor(context, source);
  const opened: DecryptedMessage[] = [];
  for (const copy of copies) {
    const name = copy.groupId === undefined ? String(copy.epoch) : keyName(copy.groupId, copy.epoch);
    const message = held.has(copy.messageId)
      ? undefined
      : await openCopy(context, idOf(source), copy, keys[name]);
    if (message !== undefined) {
      opened.push("channelId" in source ? { ...message, channelId: source.channelId } : message);
      held.add(message.id);
    }
  }
  return opened;
}

/**
 * Indexes the source's whole history, reading older archive pages until
 * the start or until `keepGoing` says to stop; call it again to resume.
 */
export async function backfill(
  context: EngineContext,
  source: HistorySource,
  keepGoing: () => Promise<boolean>,
): Promise<BackfillProgress> {
  const id = idOf(source);
  await indexEverything(context.database, id);
  const held = await loadMessages(context, id);
  await indexMessages(context.database, id, held, { everything: true });
  const known = new Set(held.map((message) => message.id));
  let indexed = held.length;
  let before = new TextDecoder().decode((await context.database.get(cursors, id)) ?? new Uint8Array());
  while (before !== finished && (await keepGoing())) {
    const copies = await page(context, source, before === "" ? undefined : before);
    const opened = await openPage(context, source, copies, known);
    await storeMessages(context, id, opened);
    indexed += opened.length;
    before = copies.at(-1)?.createdAt ?? finished;
    await context.database.put(cursors, id, utf8(before));
  }
  return { indexed, done: before === finished };
}
