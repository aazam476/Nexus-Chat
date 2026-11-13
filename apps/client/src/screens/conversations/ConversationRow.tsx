import type { Conversation } from "@nexus-chat/api-client";
import { Avatar, Badge, ListRow } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { conversationName, othersIn } from "../../messaging/conversationName";
import { mentionsUser } from "../../messaging/mentions";
import type { TimelineItem } from "../../messaging/timeline";
import { useTimeline } from "../../messaging/useConversations";
import { lastLine } from "../conversation/lastLine";

/** Props of {@link ConversationRow}. */
export interface ConversationRowProps {
  conversation: Conversation;
  selfId: string;
  /** Messages from others since the user last read it, and the last one read. */
  unread: number;
  lastReadId: number;
}

/** Unread messages from others that mention the user or everyone. */
function unreadMentions(
  items: TimelineItem[],
  conversation: Conversation,
  selfId: string,
  lastReadId: number,
): number {
  const username = conversation.members.find((member) => member.id === selfId)?.username ?? "";
  return items.filter(
    (item) =>
      (item.deliveryId ?? 0) > lastReadId &&
      !item.sender.startsWith(`@${username}@`) &&
      !item.deleted &&
      mentionsUser(item.text, selfId),
  ).length;
}

/** One conversation in the list: who, and the last message. */
export function ConversationRow({
  conversation,
  selfId,
  unread,
  lastReadId,
}: ConversationRowProps): ReactNode {
  const { t } = useTranslation();
  const { address } = useConnection();
  const timeline = useTimeline(conversation.id);
  const name = conversationName(conversation, selfId, t);
  const avatar =
    conversation.kind === "group" ? conversation.iconUrl : othersIn(conversation, selfId)[0]?.avatarUrl;
  const last = timeline?.items.at(-1);
  const mentions = unreadMentions(timeline?.items ?? [], conversation, selfId, lastReadId);
  return (
    <ListRow
      title={name}
      subtitle={last === undefined ? t("conversations.noMessages") : lastLine(last, t)}
      leading={
        <Avatar
          name={name}
          {...(avatar === undefined || address === undefined ? {} : { imageUrl: `${address}${avatar}` })}
        />
      }
      trailing={
        <>
          <Badge count={mentions} prefix="@" label={t("conversations.mentions", { count: mentions })} />
          <Badge count={unread} label={t("conversations.unread", { count: unread })} />
        </>
      }
      emphasized={unread > 0}
      onPress={() => {
        router.push({ pathname: "/conversation/[id]", params: { id: conversation.id } });
      }}
      testID={`conversation-${conversation.id}`}
    />
  );
}
