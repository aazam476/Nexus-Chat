/**
 * Where a search result is, in words and as a route: a conversation by its
 * name, or a server channel by #name and its server.
 */
import { getServer, listServerMembers, type Conversation, type UserSummary } from "@nexus-chat/api-client";
import type { TFunction } from "i18next";
import { useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { isolate } from "../i18n/bidi";
import { conversationName } from "../messaging/conversationName";
import { messengerOf, useMessaging } from "../messaging/MessagingContext";
import { useConversations } from "../messaging/useConversations";

/** A result's place: its name and where opening it leads. */
export interface Place {
  name: string;
  href:
    | { pathname: "/conversation/[id]"; params: { id: string } }
    | { pathname: "/server/[id]/channel/[channelId]"; params: { id: string; channelId: string } };
}

/** What the servers some results are in call things: channels by ID, and their members. */
interface ServerNames {
  channels: ReadonlyMap<string, string>;
  people: readonly UserSummary[];
}

/** Loads a server's channel names and members. */
async function loadServer(
  client: Parameters<typeof getServer>[0]["client"],
  serverId: string,
  t: TFunction,
): Promise<ServerNames> {
  const [details, members] = await Promise.all([
    getServer({ client, path: { serverId } }),
    listServerMembers({ client, path: { serverId } }),
  ]);
  const channels = new Map<string, string>();
  for (const channel of details.data?.channels ?? []) {
    const server = isolate(details.data?.server.name ?? "");
    channels.set(channel.id, `${t("servers.textChannel", { name: isolate(channel.name) })} · ${server}`);
  }
  return { channels, people: (members.data ?? []).map((member) => member.user) };
}

/** Names in the servers some results are in. */
function useServerNames(serverIds: readonly string[], t: TFunction): ServerNames {
  const { client } = useConnection();
  const [names, setNames] = useState<ServerNames>(() => ({ channels: new Map(), people: [] }));
  const key = [...new Set(serverIds)].sort().join(",");
  useEffect(() => {
    if (client === undefined || key === "") {
      return;
    }
    let current = true;
    void Promise.all(key.split(",").map((serverId) => loadServer(client, serverId, t))).then((loaded) => {
      if (current) {
        setNames({
          channels: new Map(loaded.flatMap((server) => [...server.channels])),
          people: loaded.flatMap((server) => server.people),
        });
      }
    });
    return () => {
      current = false;
    };
  }, [client, key, t]);
  return names;
}

/** A sender's display name, from `@name@instance`, among `people`; their username otherwise. */
function namer(people: readonly UserSummary[]): (sender: string) => string {
  return (sender) => {
    const username = sender.replace(/^@/, "").split("@")[0] ?? sender;
    const person = people.find((candidate) => candidate.username === username);
    return isolate(person?.displayName ?? username);
  };
}

/** Places of the conversations and channels results are in, and the names of their senders. */
export function usePlaces(
  ids: readonly string[],
  t: TFunction,
): { placeOf: (id: string) => Place; nameOf: (sender: string) => string } {
  const messenger = messengerOf(useMessaging());
  const { conversations } = useConversations();
  const { user } = useConnection();
  const servers = new Map(
    ids.flatMap((id) => {
      const serverId = messenger
        ?.audienceList()
        .find((audience) => audience.channelIds.includes(id))?.serverId;
      return serverId === undefined ? [] : [[id, serverId] as const];
    }),
  );
  const names = useServerNames([...servers.values()], t);
  const channelNames = names.channels;
  const byId = new Map<string, Conversation>(
    (conversations ?? []).map((conversation) => [conversation.id, conversation]),
  );
  const placeOf = (id: string): Place => {
    const serverId = servers.get(id);
    if (serverId !== undefined) {
      return {
        name: channelNames.get(id) ?? t("search.aChannel"),
        href: { pathname: "/server/[id]/channel/[channelId]", params: { id: serverId, channelId: id } },
      };
    }
    const conversation = byId.get(id);
    return {
      name:
        conversation === undefined
          ? t("search.aConversation")
          : conversationName(conversation, user?.id ?? "", t),
      href: { pathname: "/conversation/[id]", params: { id } },
    };
  };
  const people = [...(conversations ?? []).flatMap((conversation) => conversation.members), ...names.people];
  return { placeOf, nameOf: namer(people) };
}
