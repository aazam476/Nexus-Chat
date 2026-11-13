import type { Conversation } from "@nexus-chat/api-client";
import { Stack, Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useReadStates } from "../../messaging/useReadState";
import { ConversationRow } from "./ConversationRow";

/** Props of {@link ConversationList}. */
export interface ConversationListProps {
  conversations: Conversation[] | undefined;
  selfId: string;
}

/** The conversations, or why there are none to show. */
export function ConversationList({ conversations, selfId }: ConversationListProps): ReactNode {
  const { t } = useTranslation();
  const readStates = useReadStates();
  if (conversations === undefined) {
    return <Text tone="muted">{t("conversations.loading")}</Text>;
  }
  if (conversations.length === 0) {
    return <Text tone="muted">{t("conversations.empty")}</Text>;
  }
  return (
    <Stack gap="xs">
      {conversations.map((conversation) => (
        <ConversationRow
          key={conversation.id}
          conversation={conversation}
          selfId={selfId}
          unread={readStates?.get(conversation.id)?.unreadCount ?? 0}
          lastReadId={readStates?.get(conversation.id)?.lastReadId ?? 0}
        />
      ))}
    </Stack>
  );
}
