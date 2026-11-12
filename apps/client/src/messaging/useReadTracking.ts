/**
 * Read state inside one open conversation: where "new messages" starts
 * (fixed when the conversation opens, so the marker stays while the user
 * reads), and marking everything shown as read.
 */
import { useEffect, useState } from "react";

import { firstUnread } from "./timeline";
import type { TimelineView } from "./useConversations";
import { useMarkRead, useReadStates } from "./useReadState";

/** The first unread message's ID, if any; the conversation is marked read as messages show. */
export function useReadTracking(
  conversationId: string,
  timeline: TimelineView | undefined,
  self: string,
): string | undefined {
  const states = useReadStates();
  const markRead = useMarkRead();
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
