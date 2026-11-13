import { Text } from "@nexus-chat/ui";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { View } from "react-native";

import type { Payload } from "../../messaging/payload";
import type { TimelineItem } from "../../messaging/timeline";
import type { ConversationAttachments } from "../../messaging/useAttachments";
import type { OpenConversation } from "../../messaging/useConversation";
import { useTypingReporter, useTypists } from "../../messaging/useTyping";
import { ComposerBar } from "./ComposerBar";
import type { Composer } from "./useComposer";

/** Props of {@link ConversationFooter}. */
export interface ConversationFooterProps {
  conversationId: string;
  open: OpenConversation;
  timeline: TimelineItem[] | undefined;
  compose: {
    composer: Composer;
    send: (payload: Payload) => Promise<unknown>;
    attachments: ConversationAttachments;
  };
}

/** Who is typing, in words: one or two names, or "several people". */
function TypingLine({ names }: { names: string[] }): ReactNode {
  const { t } = useTranslation();
  const [first, second] = names;
  const text =
    first === undefined
      ? ""
      : second === undefined
        ? t("conversation.typingOne", { name: first })
        : names.length === 2
          ? t("conversation.typingTwo", { first, second })
          : t("conversation.typingMany");
  // Always rendered, so the composer does not jump when someone starts typing.
  return (
    <View testID="typing">
      <Text variant="small" tone="muted" live>
        {text}
      </Text>
    </View>
  );
}

/**
 * Who is typing, then the composer. Typing in the composer tells the other
 * members; it shows no animation, since it changes many times a minute.
 */
export function ConversationFooter({
  conversationId,
  open,
  timeline,
  compose,
}: ConversationFooterProps): ReactNode {
  const latest = timeline?.at(-1);
  const selfId = open.userIdOf(open.self) ?? "";
  const typists = useTypists(
    conversationId,
    selfId,
    latest === undefined ? undefined : { id: latest.id, senderId: open.userIdOf(latest.sender) },
  );
  const report = useTypingReporter(conversationId);
  const { composer, send, attachments } = compose;
  const typingComposer: Composer = {
    ...composer,
    setDraft: (text) => {
      composer.setDraft(text);
      report(text);
    },
  };
  const names = typists.map((userId) => open.nameOfId(userId) ?? userId);
  return (
    <View>
      <TypingLine names={names} />
      <ComposerBar
        composer={typingComposer}
        nameOf={open.nameOf}
        send={send}
        attachments={attachments}
        mention={{ people: open.conversation?.members ?? [], selfId }}
      />
    </View>
  );
}
