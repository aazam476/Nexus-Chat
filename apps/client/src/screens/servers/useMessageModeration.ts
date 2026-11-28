/** Reporting others' messages, and for moderators deleting them, in a channel or conversation. */
import type { ServerChannel } from "@nexus-chat/api-client";
import { router } from "expo-router";

import { useConnection } from "../../connection/ConnectionContext";
import type { TimelineItem } from "../../messaging/timeline";
import { deleteForEveryone } from "../../servers/moderationActions";
import type { MessageListProps } from "../conversation/MessageList";

/** What the message list offers for a channel's messages. */
export function useChannelModeration(
  serverId: string,
  channel: ServerChannel | undefined,
): MessageListProps["others"] {
  const { client } = useConnection();
  if (channel === undefined) {
    return undefined;
  }
  const report = (item: TimelineItem): void => {
    router.push({ pathname: "/report", params: { serverId, channelId: channel.id, messageId: item.id } });
  };
  if (client === undefined || !channel.permissions.includes("MANAGE_MESSAGES")) {
    return { report };
  }
  return {
    report,
    // The server tells every device, this one included, with a signed directive.
    removeForEveryone: (item) => {
      void deleteForEveryone(client, { serverId, channelId: channel.id }, item.id);
    },
  };
}

/** What the message list offers for a direct or group conversation's messages. */
export function conversationModeration(conversationId: string): MessageListProps["others"] {
  return {
    report: (item) => {
      router.push({ pathname: "/report", params: { conversationId, messageId: item.id } });
    },
  };
}
