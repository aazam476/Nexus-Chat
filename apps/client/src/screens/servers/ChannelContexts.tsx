import type { ReactNode } from "react";

import { AttachmentsContext, type useAttachments } from "../../messaging/useAttachments";
import { ServerEmojiContext, useChannelEmoji } from "../../servers/ServerEmoji";
import { MentionNamesContext } from "../conversation/MentionNames";
import type { useChannel } from "./useChannel";

/** What the channel's messages and composer read: attachments, offered emoji and mentioned names. */
export function ChannelContexts({
  open,
  attachments,
  serverId,
  children,
}: {
  open: ReturnType<typeof useChannel>;
  attachments: ReturnType<typeof useAttachments>;
  serverId: string;
  children: ReactNode;
}): ReactNode {
  const { people } = open;
  return (
    <AttachmentsContext value={attachments}>
      <ServerEmojiContext value={useChannelEmoji(serverId, open.channel)}>
        <MentionNamesContext
          value={{ nameOfId: people.nameOfId, selfId: people.userIdOf(people.self) ?? "" }}
        >
          {children}
        </MentionNamesContext>
      </ServerEmojiContext>
    </AttachmentsContext>
  );
}
