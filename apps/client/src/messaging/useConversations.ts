/**
 * The user's conversations and the messages of one, for the screens. The
 * list reloads when the gateway says a conversation or a relationship
 * changed; every conversation is opened on this device in the background,
 * so messages can be read and sent as soon as it is shown.
 */
import { listConversations, type Conversation } from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { useGatewayEvents } from "../realtime/GatewayContext";
import { messengerOf, useMessaging } from "./MessagingContext";
import { buildTimeline, type TimelineItem } from "./timeline";

/** The conversations, or undefined while they load (or could not be). */
export function useConversations(): { conversations: Conversation[] | undefined; reload: () => void } {
  const { client } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [conversations, setConversations] = useState<Conversation[]>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    if (event.type === "conversation_update" || event.type === "relationship_update") {
      reload();
    }
  });
  useEffect(() => {
    if (client === undefined || messenger === undefined) {
      return;
    }
    let current = true;
    void listConversations({ client }).then((result) => {
      if (current && result.data !== undefined) {
        setConversations(result.data);
        for (const conversation of result.data) {
          messenger.open(conversation.id, conversation.members).catch(() => undefined);
        }
      }
    });
    return () => {
      current = false;
    };
  }, [client, messenger, version]);
  return { conversations, reload };
}

/** What a conversation shows, updated as messages arrive. */
export function useTimeline(conversationId: string): TimelineItem[] | undefined {
  const messenger = messengerOf(useMessaging());
  const [timeline, setTimeline] = useState<TimelineItem[]>();
  useEffect(() => {
    if (messenger === undefined) {
      return;
    }
    let current = true;
    const load = (): void => {
      void messenger.messages(conversationId).then((messages) => {
        if (current) {
          setTimeline(buildTimeline(messages));
        }
      });
    };
    load();
    const stop = messenger.subscribe((changed) => {
      if (changed === conversationId) {
        load();
      }
    });
    return () => {
      current = false;
      stop();
    };
  }, [messenger, conversationId]);
  return timeline;
}
