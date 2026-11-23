/**
 * Read state in a server's channels (spec §4.5), kept on the server so the
 * user's devices agree: how far they read each channel and how many
 * messages from others came after. It reloads when a channel's messages
 * arrive on this device, another device reads further, or the server
 * changes.
 */
import { listChannelReadStates, markChannelRead, type ChannelReadState } from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import type { ReadSource } from "../messaging/useReadTracking";
import { messengerOf, useMessaging } from "../messaging/MessagingContext";
import { useGatewayEvents } from "../realtime/GatewayContext";

/** Counts the reasons to reload the server's read state. */
function useReloads(serverId: string): number {
  const messenger = messengerOf(useMessaging());
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    const data = event.data as { channelId?: unknown; serverId?: unknown } | null;
    const read = event.type === "read_update" && data?.channelId !== undefined;
    if (read || (event.type === "server_update" && data?.serverId === serverId)) {
      reload();
    }
  });
  useEffect(() => messenger?.subscribe(reload), [messenger, reload]);
  return version;
}

/** Each visible text channel's read state, by channel ID, and marking one read. */
export function useChannelReads(
  serverId: string,
): ReadSource & { states: ReadonlyMap<string, ChannelReadState> | undefined } {
  const { client } = useConnection();
  const [states, setStates] = useState<ReadonlyMap<string, ChannelReadState>>();
  const version = useReloads(serverId);
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void listChannelReadStates({ client, path: { serverId } }).then((result) => {
      if (current && result.data !== undefined) {
        setStates(new Map(result.data.map((state) => [state.channelId, state])));
      }
    });
    return () => {
      current = false;
    };
  }, [client, serverId, version]);
  const markRead = useCallback(
    (channelId: string, lastReadId: number) => {
      if (client !== undefined) {
        void markChannelRead({ client, path: { serverId, channelId }, body: { lastReadId } });
      }
    },
    [client, serverId],
  );
  return { states, markRead };
}
