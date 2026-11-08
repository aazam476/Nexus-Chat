/**
 * Keeps the gateway connected while a user is logged in, and turns its
 * events into state the screens use: the user's presence as others see it,
 * and account changes made on other devices.
 */
import type { CurrentUser } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { GatewayClient, type GatewayEvent } from "./gatewayClient";

/** The user's own presence, as the gateway reports it. */
export interface OwnPresence {
  status: string | undefined;
}

/** Connects to the gateway for the logged-in user. */
export function useGateway(token: string | undefined): OwnPresence {
  const connection = useConnection();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const { address, setUser, signOut } = connection;
  useEffect(() => {
    if (address === undefined || token === undefined) {
      return undefined;
    }
    const client = new GatewayClient({
      address,
      token,
      onEvent: (event) => {
        apply(event, { setStatus, setUser });
      },
      onSignedOut: () => void signOut(),
    });
    client.start();
    return () => {
      client.stop();
    };
  }, [address, token, setUser, signOut]);
  return { status };
}

interface Appliers {
  setStatus: (status: string) => void;
  setUser: (user: CurrentUser) => void;
}

/** Updates state for one event; unknown events are ignored (protocol §8). */
export function apply(event: GatewayEvent, appliers: Appliers): void {
  if (event.type === "presence_update" || event.type === "ready") {
    const data = event.data as { status?: string; presence?: { status: string } };
    const status = data.presence?.status ?? data.status;
    if (status !== undefined) {
      appliers.setStatus(status);
    }
  }
  if (event.type === "user_update" || event.type === "ready") {
    appliers.setUser((event.data as { user: CurrentUser }).user);
  }
}
