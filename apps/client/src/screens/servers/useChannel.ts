/**
 * One server text channel for its screen, or one thread in it: the
 * channel, its group, its people, and sending there.
 */
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
  /** In the channel, SEND_MESSAGES; in a thread, SEND_IN_THREADS. */
  mayWrite: boolean;
  /** Whether the user may start threads (CREATE_THREADS and SEND_IN_THREADS). */
  mayStartThreads: boolean;
  /** Whether the user may reply in threads that exist (SEND_IN_THREADS). */
  mayReplyInThreads: boolean;
  send: (payload: Payload) => Promise<unknown>;
}

/** Loads the channel's server and finds the channel and its audience; `thread` sends in that thread. */
export function useChannel(serverId: string, channelId: string, thread?: string): OpenChannel {
  const messenger = messengerOf(useMessaging());
  const { server } = useServer(serverId);
  const channel = server?.details.channels.find((candidate) => candidate.id === channelId);
  const audience = server?.audiences.find((candidate) => candidate.channelIds.includes(channelId));
  const people = usePeople(server?.members.map((member) => member.user) ?? noPeople);
  const send = (payload: Payload): Promise<unknown> =>
    messenger === undefined || audience === undefined
      ? Promise.reject(new Error("the channel is not ready"))
      : messenger.sendInChannel(audience, channelId, payload, thread);
  const may = (permission: ServerChannel["permissions"][number]): boolean =>
    channel?.permissions.includes(permission) ?? false;
  return {
    channel,
    audience,
    people,
    loading: server === undefined,
    mayWrite: may(thread === undefined ? "SEND_MESSAGES" : "SEND_IN_THREADS"),
    mayStartThreads: may("CREATE_THREADS") && may("SEND_IN_THREADS"),
    mayReplyInThreads: may("SEND_IN_THREADS"),
    send,
  };
}

const noPeople: ConversationPeople["members"] = [];
