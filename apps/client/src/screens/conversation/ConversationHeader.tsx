import { Heading, Link, Stack } from "@nexus-chat/ui";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../components/HttpsWarning";
import { useConnection } from "../../connection/ConnectionContext";
import { conversationName } from "../../messaging/conversationName";
import type { Messenger } from "../../messaging/messenger";
import type { OpenConversation } from "../../messaging/useConversation";
import { IdentityWarning } from "./IdentityWarning";

/** Props of {@link ConversationHeader}. */
export interface ConversationHeaderProps {
  open: OpenConversation;
  messenger: Messenger | undefined;
}

/** The way back, the conversation's name, and any identity warning. */
export function ConversationHeader({ open, messenger }: ConversationHeaderProps): ReactNode {
  const { t } = useTranslation();
  const { user } = useConnection();
  const { conversation } = open;
  return (
    <Stack gap="xs">
      <HttpsWarning />
      <Link
        label={t("conversation.back")}
        onPress={() => {
          router.back();
        }}
        testID="conversation-back"
      />
      <Heading>
        {conversation === undefined || user === undefined ? "" : conversationName(conversation, user.id, t)}
      </Heading>
      {conversation?.kind === "group" ? (
        <Link
          label={t("conversation.groupSettings")}
          onPress={() => {
            router.push({ pathname: "/group/[id]", params: { id: conversation.id } });
          }}
          testID="open-group-settings"
        />
      ) : null}
      {messenger === undefined || conversation === undefined ? null : (
        <IdentityWarning messenger={messenger} members={conversation.members} />
      )}
    </Stack>
  );
}
