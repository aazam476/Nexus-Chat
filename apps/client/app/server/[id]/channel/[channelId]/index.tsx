import { ConversationLayout, Text } from "@nexus-chat/ui";
import { useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useTimeline } from "../../../../../src/messaging/useConversations";
import { AttachmentsContext, useAttachments } from "../../../../../src/messaging/useAttachments";
import { ConversationFooter } from "../../../../../src/screens/conversation/ConversationFooter";
import { MentionNamesContext } from "../../../../../src/screens/conversation/MentionNames";
import { MessageList } from "../../../../../src/screens/conversation/MessageList";
import { useComposer } from "../../../../../src/screens/conversation/useComposer";
import { ChannelHeader } from "../../../../../src/screens/servers/ChannelHeader";
import { useChannel } from "../../../../../src/screens/servers/useChannel";

/**
 * A server text channel: its messages and the composer, end-to-end
 * encrypted in the channel's audience group. Someone who may only read
 * sees why there is no composer.
 */
export default function ChannelScreen(): ReactNode {
  const { t } = useTranslation();
  const { id, channelId } = useLocalSearchParams<{ id: string; channelId: string }>();
  const open = useChannel(id, channelId);
  const timeline = useTimeline(channelId)?.items;
  const composer = useComposer();
  // Attachments travel with the audience's group, like a group DM's.
  const attachments = useAttachments(open.audience?.audienceId ?? "");
  const { people, send } = open;
  const footer = open.mayWrite ? (
    <ConversationFooter
      conversationId={channelId}
      open={people}
      timeline={timeline}
      compose={{ composer, send, attachments }}
      typing={false}
    />
  ) : (
    <Text tone="muted">{open.channel === undefined ? "" : t("servers.readOnly")}</Text>
  );
  return (
    <AttachmentsContext value={attachments}>
      <MentionNamesContext value={{ nameOfId: people.nameOfId, selfId: people.userIdOf(people.self) ?? "" }}>
        <ConversationLayout
          header={<ChannelHeader channel={open.channel} loading={open.loading} />}
          footer={footer}
        >
          <MessageList timeline={timeline} people={people} composer={composer} send={send} />
        </ConversationLayout>
      </MentionNamesContext>
    </AttachmentsContext>
  );
}
