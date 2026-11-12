/**
 * One gateway connection for the whole app while a user is logged in.
 * Screens read the user's presence from it, and features such as
 * messaging subscribe to its events, so no screen opens a socket of its own.
 */
import { createContext, use, useEffect, useMemo, useRef, type ReactNode } from "react";

import { useConnection } from "../connection/ConnectionContext";
import type { GatewayEvent } from "./gatewayClient";
import { useGateway } from "./useGateway";

type Listener = (event: GatewayEvent) => void;

interface Gateway {
  /** The user's presence as others see it, once the gateway reported it. */
  status: string | undefined;
  subscribe: (listener: Listener) => () => void;
  reportTyping: (conversationId: string) => void;
}

const GatewayContext = createContext<Gateway | undefined>(undefined);

/** Props of {@link GatewayProvider}. */
export interface GatewayProviderProps {
  children: ReactNode;
}

/** Holds the gateway connection for everything inside it. */
export function GatewayProvider({ children }: GatewayProviderProps): ReactNode {
  const { sessionToken } = useConnection();
  const listenersRef = useRef(new Set<Listener>());
  const presence = useGateway(sessionToken, (event) => {
    for (const listener of listenersRef.current) {
      listener(event);
    }
  });
  const { reportTyping } = presence;
  const value = useMemo<Gateway>(
    () => ({
      status: presence.status,
      reportTyping,
      subscribe: (listener) => {
        listenersRef.current.add(listener);
        return () => {
          listenersRef.current.delete(listener);
        };
      },
    }),
    [presence.status, reportTyping],
  );
  return <GatewayContext value={value}>{children}</GatewayContext>;
}

function useGatewayContext(): Gateway {
  const gateway = use(GatewayContext);
  if (gateway === undefined) {
    throw new Error("useGatewayStatus and useGatewayEvents need a GatewayProvider");
  }
  return gateway;
}

/** The user's presence as the gateway last reported it. */
export function useGatewayStatus(): string | undefined {
  return useGatewayContext().status;
}

/** Calls `handler` for every gateway event while the component is mounted. */
export function useGatewayEvents(handler: Listener): void {
  const { subscribe } = useGatewayContext();
  const handlerRef = useRef(handler);
  useEffect(() => {
    handlerRef.current = handler;
  });
  useEffect(
    () =>
      subscribe((event) => {
        handlerRef.current(event);
      }),
    [subscribe],
  );
}

/** Tells a conversation's other members that the user is typing. */
export function useReportTyping(): (conversationId: string) => void {
  return useGatewayContext().reportTyping;
}
