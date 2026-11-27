import { ConversationLayout, Text } from "@nexus-chat/ui";
import { useLocalSearchParams } from "expo-router";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { useReadTracking } from "../../../../../src/messaging/useReadTracking";
import { useChannelReads } from "../../../../../src/servers/useChannelReads";
import { useAttachments } from "../../../../../src/messaging/useAttachments";
import { ConversationFooter } from "../../../../../src/screens/conversation/ConversationFooter";
import { MessageList } from "../../../../../src/screens/conversation/MessageList";
import { useComposer } from "../../../../../src/screens/conversation/useComposer";
import { ChannelContexts } from "../../../../../src/screens/servers/ChannelContexts";
import { ChannelHeader } from "../../../../../src/screens/servers/ChannelHeader";
import { useChannel } from "../../../../../src/screens/servers/useChannel";
import { useChannelView } from "../../../../../src/screens/servers/useChannelView";

/**
 * A server text channel: its messages and the composer, end-to-end
 * encrypted in the channel's audience group. Someone who may only read
 * sees why there is no composer.
 */
export default function ChannelScreen(): ReactNode {
  const { t } = useTranslation();
  const { id, channelId } = useLocalSearchParams<{ id: string; channelId: string }>();
  const open = useChannel(id, channelId);
  const { view, thread } = useChannelView(id, channelId, open);
  const timeline = view?.items;
  const composer = useComposer();
  // Attachments travel with the audience's group, like a group DM's.
  const attachments = useAttachments(open.audience?.audienceId ?? "");
  const { people, send } = open;
  const firstUnreadId = useReadTracking(channelId, view, { self: people.self, source: useChannelReads(id) });
  const footer = open.mayWrite ? (
    <ConversationFooter
      typingIn={{ channelId }}
      open={people}
      timeline={timeline}
      compose={{ composer, send, attachments }}
    />
  ) : (
    <Text tone="muted">{open.channel === undefined ? "" : t("servers.readOnly")}</Text>
  );
  return (
    <ChannelContexts open={open} attachments={attachments} serverId={id}>
      <ConversationLayout
        header={<ChannelHeader channel={open.channel} loading={open.loading} />}
        footer={footer}
      >
        <MessageList
          timeline={timeline}
          people={people}
          composer={composer}
          send={send}
          firstUnreadId={firstUnreadId}
          thread={thread}
        />
      </ConversationLayout>
    </ChannelContexts>
  );
}
