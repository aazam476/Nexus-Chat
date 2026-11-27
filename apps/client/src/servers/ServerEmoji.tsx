/**
 * A server's custom emoji, for the screens that offer them: the composer's
 * suggestions and the reactions on messages. Only members who may use
 * custom emoji in the channel are offered them; everyone sees them drawn.
 */
import { listServerEmoji, type ServerChannel, type ServerEmoji } from "@nexus-chat/api-client";
import { createContext, use, useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { useGatewayEvents } from "../realtime/GatewayContext";

/** The emoji the open channel offers; empty outside a server channel. */
export const ServerEmojiContext = createContext<readonly ServerEmoji[]>([]);

/** The emoji the open channel offers. */
export function useOfferedEmoji(): readonly ServerEmoji[] {
  return use(ServerEmojiContext);
}

/** The server's emoji, by name, reloaded when the server changes; undefined until loaded. */
export function useServerEmoji(serverId: string): { emoji: ServerEmoji[] | undefined; reload: () => void } {
  const { client } = useConnection();
  const [emoji, setEmoji] = useState<ServerEmoji[]>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    if (
      event.type === "server_update" &&
      (event.data as { serverId?: unknown } | null)?.serverId === serverId
    ) {
      reload();
    }
  });
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void listServerEmoji({ client, path: { serverId } }).then((result) => {
      if (current && result.data !== undefined) {
        setEmoji(result.data);
      }
    });
    return () => {
      current = false;
    };
  }, [client, serverId, version]);
  return { emoji, reload };
}

/**
 * The emoji a channel offers: everyone sees custom emoji, but only those
 * who may use them there (USE_CUSTOM_EMOJI) are offered them.
 */
export function useChannelEmoji(
  serverId: string,
  channel: ServerChannel | undefined,
): readonly ServerEmoji[] {
  const { emoji } = useServerEmoji(serverId);
  return channel?.permissions.includes("USE_CUSTOM_EMOJI") === true ? (emoji ?? []) : [];
}
