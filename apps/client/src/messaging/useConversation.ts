/**
 * One conversation for its screen: its details from the server, opened on
 * this device, and how to name its members.
 */
import { getConversation, type Conversation, type UserSummary } from "@nexus-chat/api-client";
import { useEffect, useMemo, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { hostOf } from "../instance/address";
import { useGatewayEvents } from "../realtime/GatewayContext";
import { isolate } from "../i18n/bidi";
import { messengerOf, useMessaging } from "./MessagingContext";

/** The people of a conversation or server channel, and how to name them. */
export interface ConversationPeople {
  /** Everyone who can be mentioned there. */
  members: UserSummary[];
  /** The signed-in user's `@name@instance`. */
  self: string;
  nameOf: (user: string) => string;
  /** A member's user ID from their `@name@instance`. */
  userIdOf: (user: string) => string | undefined;
  /** A member's display name, isolated for right-to-left text, by user ID. */
  nameOfId: (userId: string) => string | undefined;
}

/** The conversation and the display name of each member's `@name@instance`. */
export interface OpenConversation extends ConversationPeople {
  conversation: Conversation | undefined;
}

/** Names the members, by the address their devices use and by user ID. */
export function usePeople(members: UserSummary[]): ConversationPeople {
  const { user, instance, address } = useConnection();
  const domain = instance?.info.domain ?? hostOf(address ?? "http://localhost");
  const names = useMemo(
    () => new Map(members.map((member) => [`@${member.username}@${domain}`, member.displayName])),
    [members, domain],
  );
  return {
    members,
    self: `@${user?.username ?? ""}@${domain}`,
    nameOf: (sender) => isolate(names.get(sender) ?? sender),
    userIdOf: (sender) => members.find((member) => `@${member.username}@${domain}` === sender)?.id,
    nameOfId: (userId) => {
      const member = members.find((candidate) => candidate.id === userId);
      return member === undefined ? undefined : isolate(member.displayName);
    },
  };
}

const noMembers: UserSummary[] = [];

/** Counts conversation_update events for the conversation, to reload it. */
function useChangeCount(conversationId: string): number {
  const [version, setVersion] = useState(0);
  useGatewayEvents((event) => {
    const changed = (event.data as { conversationId?: unknown } | null)?.conversationId;
    if (event.type === "conversation_update" && changed === conversationId) {
      setVersion((count) => count + 1);
    }
  });
  return version;
}

/** Loads the conversation and makes sure this device is in it. */
export function useConversation(conversationId: string): OpenConversation {
  const { client } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [conversation, setConversation] = useState<Conversation>();
  const version = useChangeCount(conversationId);
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void getConversation({ client, path: { conversationId } }).then((result) => {
      if (current && result.data !== undefined) {
        setConversation(result.data);
        messenger?.open(conversationId, result.data.members).catch(() => undefined);
      }
    });
    return () => {
      current = false;
    };
  }, [client, messenger, conversationId, version]);
  return { conversation, ...usePeople(conversation?.members ?? noMembers) };
}
