import { ConversationLayout, Text } from "@nexus-chat/ui";
import { useLocalSearchParams } from "expo-router";
import { useEffect, useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { messengerOf, useMessaging } from "../../../../../../src/messaging/MessagingContext";
import { threadItems } from "../../../../../../src/messaging/timeline";
import { useAttachments } from "../../../../../../src/messaging/useAttachments";
import { useTimeline } from "../../../../../../src/messaging/useConversations";
import { ConversationFooter } from "../../../../../../src/screens/conversation/ConversationFooter";
import { MessageList } from "../../../../../../src/screens/conversation/MessageList";
import { useComposer } from "../../../../../../src/screens/conversation/useComposer";
import { ChannelContexts } from "../../../../../../src/screens/servers/ChannelContexts";
import { ThreadHeader } from "../../../../../../src/screens/servers/ThreadHeader";
import { useChannel, type OpenChannel } from "../../../../../../src/screens/servers/useChannel";
import { useThreadFollow } from "../../../../../../src/servers/useThreadFollow";

/** The thread's messages, reading its archive for replies this device has not got. */
function useThreadTimeline(open: OpenChannel, channelId: string, rootId: string) {
  const messenger = messengerOf(useMessaging());
  const { audience } = open;
  useEffect(() => {
    if (messenger !== undefined && audience !== undefined) {
      messenger.readThread(audience, channelId, rootId).catch(() => undefined);
    }
  }, [messenger, audience, channelId, rootId]);
  const whole = useTimeline(channelId);
  return useMemo(() => (whole === undefined ? undefined : threadItems(whole.items, rootId)), [whole, rootId]);
}

/**
 * A thread (spec §4.5): the message it is on, then its replies, with its
 * own composer. It fills the screen, and the way back leads to the channel.
 */
export default function ThreadScreen(): ReactNode {
  const { t } = useTranslation();
  const { id, channelId, rootId } = useLocalSearchParams<{ id: string; channelId: string; rootId: string }>();
  const open = useChannel(id, channelId, rootId);
  const timeline = useThreadTimeline(open, channelId, rootId);
  const composer = useComposer();
  const attachments = useAttachments(open.audience?.audienceId ?? "");
  const follow = useThreadFollow(id, channelId, rootId);
  const replies = (timeline ?? []).filter((item) => item.id !== rootId).length;
  const mayReply = open.mayWrite && (replies > 0 || open.mayStartThreads);
  const footer = mayReply ? (
    <ConversationFooter
      typingIn={{ channelId }}
      open={open.people}
      timeline={timeline}
      compose={{ composer, send: open.send, attachments }}
    />
  ) : (
    <Text tone="muted">{open.channel === undefined ? "" : t("threads.readOnly")}</Text>
  );
  return (
    <ChannelContexts open={open} attachments={attachments} serverId={id}>
      <ConversationLayout header={<ThreadHeader follow={follow} />} footer={footer}>
        {timeline !== undefined && replies === 0 ? <Text tone="muted">{t("threads.noReplies")}</Text> : null}
        <MessageList timeline={timeline} people={open.people} composer={composer} send={open.send} />
      </ConversationLayout>
    </ChannelContexts>
  );
}
