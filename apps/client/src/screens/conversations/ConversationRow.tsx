import type { Conversation } from "@nexus-chat/api-client";
import { Avatar, ListRow } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { conversationName, othersIn } from "../../messaging/conversationName";
import { useTimeline } from "../../messaging/useConversations";
import { lastLine } from "../conversation/lastLine";

/** Props of {@link ConversationRow}. */
export interface ConversationRowProps {
  conversation: Conversation;
  selfId: string;
}

/** One conversation in the list: who, and the last message. */
export function ConversationRow({ conversation, selfId }: ConversationRowProps): ReactNode {
  const { t } = useTranslation();
  const { address } = useConnection();
  const timeline = useTimeline(conversation.id);
  const name = conversationName(conversation, selfId, t);
  const avatar = othersIn(conversation, selfId)[0]?.avatarUrl;
  const last = timeline?.at(-1);
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
      onPress={() => {
        router.push({ pathname: "/conversation/[id]", params: { id: conversation.id } });
      }}
      accessibilityLabel={t("conversations.open", { name })}
      testID={`conversation-${conversation.id}`}
    />
  );
}
