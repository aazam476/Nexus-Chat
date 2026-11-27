/** A channel's messages as its screen shows them, and opening their threads. */
import { router } from "expo-router";
import { useMemo } from "react";

import { channelItems, type TimelineItem } from "../../messaging/timeline";
import { useTimeline, type TimelineView } from "../../messaging/useConversations";
import type { MessageListProps } from "../conversation/MessageList";
import type { OpenChannel } from "./useChannel";

/**
 * The channel's own messages, without replies in threads, each with its
 * thread's reply count; and the thread actions: someone may reply in a
 * message's thread when it has one and they may reply in threads, or when
 * they may start one.
 */
export function useChannelView(
  serverId: string,
  channelId: string,
  open: Pick<OpenChannel, "mayStartThreads" | "mayReplyInThreads">,
): { view: TimelineView | undefined; thread: MessageListProps["thread"] } {
  const whole = useTimeline(channelId);
  const view = useMemo(
    () => (whole === undefined ? undefined : { ...whole, items: channelItems(whole.items) }),
    [whole],
  );
  const thread = {
    open: (item: TimelineItem) => {
      router.push({
        pathname: "/server/[id]/channel/[channelId]/thread/[rootId]",
        params: { id: serverId, channelId, rootId: item.id },
      });
    },
    mayReply: (item: TimelineItem) =>
      open.mayStartThreads || (open.mayReplyInThreads && item.replies !== undefined),
  };
  return { view, thread };
}
