/**
 * Read state (spec §4.5): how far the user has read each conversation and
 * how many messages from others came after, kept on the server so every
 * device agrees. It reloads when messages arrive or another device of the
 * user reads further.
 */
import { listReadStates, markConversationRead, type ReadState } from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { useGatewayEvents } from "../realtime/GatewayContext";

const reloadOn = new Set(["mls_messages", "read_update", "conversation_update"]);

/** Each conversation's read state, by conversation ID; undefined while loading. */
export function useReadStates(): ReadonlyMap<string, ReadState> | undefined {
  const { client } = useConnection();
  const [states, setStates] = useState<ReadonlyMap<string, ReadState>>();
  const [version, setVersion] = useState(0);
  useGatewayEvents((event) => {
    if (reloadOn.has(event.type)) {
      setVersion((count) => count + 1);
    }
  });
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void listReadStates({ client }).then((result) => {
      if (current && result.data !== undefined) {
        setStates(new Map(result.data.map((state) => [state.conversationId, state])));
      }
    });
    return () => {
      current = false;
    };
  }, [client, version]);
  return states;
}

/** Marks a conversation read up to a delivered message; failures are retried by the next call. */
export function useMarkRead(): (conversationId: string, lastReadId: number) => void {
  const { client } = useConnection();
  return useCallback(
    (conversationId: string, lastReadId: number) => {
      if (client !== undefined) {
        void markConversationRead({ client, path: { conversationId }, body: { lastReadId } });
      }
    },
    [client],
  );
}
