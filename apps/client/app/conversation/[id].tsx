import { ConversationLayout } from "@nexus-chat/ui";
import { useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";

import { messengerOf, useMessaging } from "../../src/messaging/MessagingContext";
import type { Payload } from "../../src/messaging/payload";
import { useConversation } from "../../src/messaging/useConversation";
import { useTimeline } from "../../src/messaging/useConversations";
import { ConversationFooter } from "../../src/screens/conversation/ConversationFooter";
import { ConversationHeader } from "../../src/screens/conversation/ConversationHeader";
import { MessageList } from "../../src/screens/conversation/MessageList";
import { useComposer } from "../../src/screens/conversation/useComposer";

/** One conversation: its messages and the composer, end-to-end encrypted. */
export default function ConversationScreen(): ReactNode {
  const { id } = useLocalSearchParams<{ id: string }>();
  const messenger = messengerOf(useMessaging());
  const open = useConversation(id);
  const timeline = useTimeline(id);
  const composer = useComposer();
  const send = (payload: Payload): Promise<unknown> =>
    messenger === undefined
      ? Promise.reject(new Error("messaging is not ready"))
      : messenger.send(id, payload);
  return (
    <ConversationLayout
      header={<ConversationHeader open={open} messenger={messenger} />}
      footer={
        <ConversationFooter
          conversationId={id}
          open={open}
          timeline={timeline}
          compose={{ composer, send }}
        />
      }
    >
      <MessageList
        timeline={timeline}
        people={{ self: open.self, nameOf: open.nameOf }}
        composer={composer}
        send={send}
      />
    </ConversationLayout>
  );
}
