/**
 * Keeping the search index shared between the user's devices (spec §4.6):
 * soon after messaging starts, and every ten minutes after, a device adds
 * the chunks its other devices shared and shares its own changes. The
 * desktop app is the heavy indexer: it indexes the whole history of every
 * conversation and channel it keeps, so phones can fetch those chunks
 * instead of downloading old messages themselves.
 */
import { useEffect } from "react";

import type { Messenger } from "../messaging/messenger";

/** How often the index is shared, in ms. */
const every = 10 * 60_000;

/** Whether this is the desktop app, which indexes everything. */
function isDesktop(): boolean {
  return (globalThis as { nexusDesktop?: unknown }).nexusDesktop !== undefined;
}

/** Indexes the whole history of every conversation and channel the device keeps. */
async function indexEverything(messenger: Messenger, conversationIds: readonly string[]): Promise<void> {
  const channelIds = messenger.audienceList().flatMap((audience) => audience.channelIds);
  for (const id of [...conversationIds, ...channelIds]) {
    await messenger.searchOlder(id, () => Promise.resolve(true));
  }
}

/** Shares the index while messaging runs; `conversationIds` are the user's conversations, for the desktop app. */
export function useIndexSync(messenger: Messenger | undefined, conversationIds: readonly string[]): void {
  const key = conversationIds.join(",");
  useEffect(() => {
    if (messenger === undefined) {
      return;
    }
    const run = async (): Promise<void> => {
      if (isDesktop()) {
        await indexEverything(messenger, key === "" ? [] : key.split(","));
      }
      await messenger.syncSearchIndex();
    };
    const first = setTimeout(() => void run().catch(() => undefined), 5000);
    const timer = setInterval(() => void run().catch(() => undefined), every);
    return () => {
      clearTimeout(first);
      clearInterval(timer);
    };
  }, [messenger, key]);
}
