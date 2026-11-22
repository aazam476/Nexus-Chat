/**
 * The user's servers for the screens. Each server is loaded with its
 * members, and the groups of its channels' audiences are opened on this
 * device in the background, so a channel can be read and written as soon
 * as it is shown. Everything reloads when the gateway says the server
 * changed (`server_update`), which is also when this user may have gained
 * or lost a channel.
 */
import {
  getServer,
  listServerMembers,
  listServers,
  type Client,
  type Server,
  type ServerDetails,
  type ServerMember,
} from "@nexus-chat/api-client";
import { useCallback, useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import type { Audience } from "../messaging/engine/channels";
import { messengerOf, useMessaging } from "../messaging/MessagingContext";
import type { Messenger } from "../messaging/messenger";
import { useGatewayEvents } from "../realtime/GatewayContext";
import { audiencesOf } from "./audiences";

/** One server as this user sees it, with its members and audiences. */
export interface LoadedServer {
  details: ServerDetails;
  members: ServerMember[];
  audiences: Audience[];
}

/** The server a gateway event names, if it is a server_update. */
function changedServer(event: { type: string; data: unknown }): string | undefined {
  if (event.type !== "server_update") {
    return undefined;
  }
  const serverId = (event.data as { serverId?: unknown } | null)?.serverId;
  return typeof serverId === "string" ? serverId : undefined;
}

/** Loads a server, or undefined when the user is not (or no longer) in it. */
export async function loadServer(client: Client, serverId: string): Promise<LoadedServer | undefined> {
  const [details, members] = await Promise.all([
    getServer({ client, path: { serverId } }),
    listServerMembers({ client, path: { serverId } }),
  ]);
  if (details.data === undefined || members.data === undefined) {
    return undefined;
  }
  return { details: details.data, members: members.data, audiences: audiencesOf(details.data, members.data) };
}

/** Opens every audience of the server on this device, in the background. */
export function openAudiences(messenger: Messenger, server: LoadedServer): void {
  const people = server.members.map((member) => member.user);
  for (const audience of server.audiences) {
    messenger.openAudience(audience, people).catch(() => undefined);
  }
}

/** The servers the user is in, or undefined while they load. */
export function useServers(): { servers: Server[] | undefined; reload: () => void } {
  const { client } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [servers, setServers] = useState<Server[]>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    if (changedServer(event) !== undefined) {
      reload();
    }
  });
  useEffect(() => {
    if (client === undefined || messenger === undefined) {
      return;
    }
    let current = true;
    void listServers({ client }).then(async (result) => {
      if (!current || result.data === undefined) {
        return;
      }
      setServers(result.data);
      for (const server of result.data) {
        // Opening a group is harmless even if the screen has gone meanwhile.
        const loaded = await loadServer(client, server.id);
        if (loaded !== undefined) {
          openAudiences(messenger, loaded);
        }
      }
    });
    return () => {
      current = false;
    };
  }, [client, messenger, version]);
  return { servers, reload };
}

/** One server for its screens: loaded, kept up to date, or gone (null) once the user leaves it. */
export function useServer(serverId: string): { server: LoadedServer | null | undefined; reload: () => void } {
  const { client } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [server, setServer] = useState<LoadedServer | null>();
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => {
    setVersion((count) => count + 1);
  }, []);
  useGatewayEvents((event) => {
    if (changedServer(event) === serverId) {
      reload();
    }
  });
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    let current = true;
    void loadServer(client, serverId).then((loaded) => {
      if (current) {
        setServer(loaded ?? null);
        if (loaded !== undefined && messenger !== undefined) {
          openAudiences(messenger, loaded);
        }
      }
    });
    return () => {
      current = false;
    };
  }, [client, messenger, serverId, version]);
  return { server, reload };
}
