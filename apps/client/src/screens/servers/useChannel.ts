/** One server text channel for its screen: the channel, its group, its people, and sending. */
import type { ServerChannel } from "@nexus-chat/api-client";

import type { Audience } from "../../messaging/engine/channels";
import { messengerOf, useMessaging } from "../../messaging/MessagingContext";
import type { Payload } from "../../messaging/payload";
import { usePeople, type ConversationPeople } from "../../messaging/useConversation";
import { useServer } from "../../servers/useServers";

/** What the channel screen needs. */
export interface OpenChannel {
  channel: ServerChannel | undefined;
  audience: Audience | undefined;
  people: ConversationPeople;
  /** Still loading the server, as opposed to the channel being gone. */
  loading: boolean;
  mayWrite: boolean;
  send: (payload: Payload) => Promise<unknown>;
}

/** Loads the channel's server and finds the channel and its audience. */
export function useChannel(serverId: string, channelId: string): OpenChannel {
  const messenger = messengerOf(useMessaging());
  const { server } = useServer(serverId);
  const channel = server?.details.channels.find((candidate) => candidate.id === channelId);
  const audience = server?.audiences.find((candidate) => candidate.channelIds.includes(channelId));
  const people = usePeople(server?.members.map((member) => member.user) ?? noPeople);
  const send = (payload: Payload): Promise<unknown> =>
    messenger === undefined || audience === undefined
      ? Promise.reject(new Error("the channel is not ready"))
      : messenger.sendInChannel(audience, channelId, payload);
  return {
    channel,
    audience,
    people,
    loading: server === undefined,
    mayWrite: channel?.permissions.includes("SEND_MESSAGES") ?? false,
    send,
  };
}

const noPeople: ConversationPeople["members"] = [];
