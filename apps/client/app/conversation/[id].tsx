import { ConversationLayout, Heading, Link, Stack } from "@nexus-chat/ui";
import { router, useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { HttpsWarning } from "../../src/components/HttpsWarning";
import { useConnection } from "../../src/connection/ConnectionContext";
import { conversationName } from "../../src/messaging/conversationName";
import { messengerOf, useMessaging } from "../../src/messaging/MessagingContext";
import type { Payload } from "../../src/messaging/payload";
import { useConversation } from "../../src/messaging/useConversation";
import { useTimeline } from "../../src/messaging/useConversations";
import { ComposerBar } from "../../src/screens/conversation/ComposerBar";
import { IdentityWarning } from "../../src/screens/conversation/IdentityWarning";
import { MessageList } from "../../src/screens/conversation/MessageList";
import { useComposer } from "../../src/screens/conversation/useComposer";

/** One conversation: its messages and the composer, end-to-end encrypted. */
export default function ConversationScreen(): ReactNode {
  const { t } = useTranslation();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { user } = useConnection();
  const messenger = messengerOf(useMessaging());
  const { conversation, self, nameOf } = useConversation(id);
  const timeline = useTimeline(id);
  const composer = useComposer();
  const send = (payload: Payload): Promise<unknown> =>
    messenger === undefined
      ? Promise.reject(new Error("messaging is not ready"))
      : messenger.send(id, payload);
  const header = (
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
      {messenger === undefined || conversation === undefined ? null : (
        <IdentityWarning messenger={messenger} members={conversation.members} />
      )}
    </Stack>
  );
  return (
    <ConversationLayout
      header={header}
      footer={<ComposerBar composer={composer} nameOf={nameOf} send={send} />}
    >
      <MessageList timeline={timeline} people={{ self, nameOf }} composer={composer} send={send} />
    </ConversationLayout>
  );
}
