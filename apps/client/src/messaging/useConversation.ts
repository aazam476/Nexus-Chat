/**
 * One conversation for its screen: its details from the server, opened on
 * this device, and how to name its members.
 */
import { getConversation, type Conversation } from "@nexus-chat/api-client";
import { useEffect, useMemo, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { hostOf } from "../instance/address";
import { isolate } from "../i18n/bidi";
import { messengerOf, useMessaging } from "./MessagingContext";

/** The conversation and the display name of each member's `@name@instance`. */
export interface OpenConversation {
  conversation: Conversation | undefined;
  /** The signed-in user's `@name@instance`. */
  self: string;
  nameOf: (user: string) => string;
}

/** Loads the conversation and makes sure this device is in it. */
export function useConversation(conversationId: string): OpenConversation {
  const { client, user, instance, address } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [conversation, setConversation] = useState<Conversation>();
  useEffect(() => {
    if (client === undefined || messenger === undefined) {
      return;
    }
    let current = true;
    void getConversation({ client, path: { conversationId } }).then((result) => {
      if (current && result.data !== undefined) {
        setConversation(result.data);
        messenger.open(conversationId, result.data.members).catch(() => undefined);
      }
    });
    return () => {
      current = false;
    };
  }, [client, messenger, conversationId]);
  const domain = instance?.info.domain ?? hostOf(address ?? "http://localhost");
  const names = useMemo(
    () =>
      new Map(
        (conversation?.members ?? []).map((member) => [`@${member.username}@${domain}`, member.displayName]),
      ),
    [conversation, domain],
  );
  return {
    conversation,
    self: `@${user?.username ?? ""}@${domain}`,
    nameOf: (sender) => isolate(names.get(sender) ?? sender),
  };
}
