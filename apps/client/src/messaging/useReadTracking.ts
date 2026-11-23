/**
 * Read state inside one open conversation: where "new messages" starts
 * (fixed when the conversation opens, so the marker stays while the user
 * reads), and marking everything shown as read.
 */
import { useEffect, useState } from "react";

import { firstUnread } from "./timeline";
import type { TimelineView } from "./useConversations";

/** Where read state comes from: conversations' or a server's channels'. */
export interface ReadSource {
  /** How far the user has read each conversation or channel, by ID; undefined while loading. */
  states: ReadonlyMap<string, { lastReadId: number }> | undefined;
  markRead: (id: string, lastReadId: number) => void;
}

/** The first unread message's ID, if any; the conversation or channel is marked read as messages show. */
export function useReadTracking(
  conversationId: string,
  timeline: TimelineView | undefined,
  read: { self: string; source: ReadSource },
): string | undefined {
  const { self, source } = read;
  const { states, markRead } = source;
  const state = states?.get(conversationId);
  // The position when the conversation opened; later reads do not move the marker.
  const [openedAt, setOpenedAt] = useState<number>();
  if (openedAt === undefined && state !== undefined) {
    setOpenedAt(state.lastReadId);
  }
  const newest = timeline?.newestDeliveryId ?? 0;
  const lastReadId = state?.lastReadId;
  useEffect(() => {
    if (lastReadId !== undefined && newest > lastReadId) {
      markRead(conversationId, newest);
    }
  }, [conversationId, lastReadId, newest, markRead]);
  if (openedAt === undefined || timeline === undefined) {
    return undefined;
  }
  return firstUnread(timeline.items, openedAt, self)?.id;
}
