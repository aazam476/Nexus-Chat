/**
 * Keeps the gateway connected while a user is logged in, and turns its
 * events into state the screens use: the user's presence as others see it,
 * and account changes made on other devices.
 */
import type { CurrentUser } from "@nexus-chat/api-client";
import { useCallback, useEffect, useRef, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { GatewayClient, type GatewayEvent } from "./gatewayClient";

/** The user's own presence, as the gateway reports it. */
export interface OwnPresence {
  status: string | undefined;
  /** Sends a typing frame for the conversation while connected. */
  reportTyping: (conversationId: string) => void;
}

/**
 * Connects to the gateway for the logged-in user. `onEvent` hears every
 * event too, for features such as messaging; it may change between renders.
 */
export function useGateway(token: string | undefined, onEvent?: (event: GatewayEvent) => void): OwnPresence {
  const connection = useConnection();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const listenerRef = useRef(onEvent);
  const clientRef = useRef<GatewayClient | undefined>(undefined);
  useEffect(() => {
    listenerRef.current = onEvent;
  });
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
        listenerRef.current?.(event);
      },
      onSignedOut: () => void signOut(),
    });
    client.start();
    clientRef.current = client;
    return () => {
      client.stop();
      clientRef.current = undefined;
    };
  }, [address, token, setUser, signOut]);
  const reportTyping = useCallback((conversationId: string) => {
    clientRef.current?.reportTyping(conversationId);
  }, []);
  return { status, reportTyping };
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
