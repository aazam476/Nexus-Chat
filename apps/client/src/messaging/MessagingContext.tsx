/**
 * Messaging for every screen: the provider starts it for the signed-in
 * user and applies new MLS messages as the gateway announces them.
 */
import { createContext, use, type ReactNode } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { useGatewayEvents } from "../realtime/GatewayContext";
import { openDatabase } from "../storage/database";
import { createMessagingApi } from "./engine/serverApi";
import type { Messenger } from "./messenger";
import { useMessagingState, type Messaging, type MessagingEnvironment } from "./useMessagingState";

/** Messaging; tests provide a value directly. */
export const MessagingContext = createContext<Messaging | undefined>(undefined);

const platform: MessagingEnvironment = { openDatabase, createApi: createMessagingApi };

/** Props of {@link MessagingProvider}. */
export interface MessagingProviderProps {
  /** Where the database and server calls come from; the platform's by default. */
  environment?: MessagingEnvironment;
  children: ReactNode;
}

/** The ID of the conversation a gateway event is about, if any. */
function conversationOf(data: unknown): string | undefined {
  if (typeof data === "object" && data !== null && "conversationId" in data) {
    const { conversationId } = data;
    return typeof conversationId === "string" ? conversationId : undefined;
  }
  return undefined;
}

/** Starts messaging for the signed-in user and keeps conversations up to date. */
export function MessagingProvider({ environment = platform, children }: MessagingProviderProps): ReactNode {
  const messaging = useMessagingState(useConnection(), environment);
  const messenger = messengerOf(messaging);
  useGatewayEvents((event) => {
    const conversationId = conversationOf(event.data);
    if (event.type === "mls_messages" && messenger !== undefined && conversationId !== undefined) {
      // A conversation this device is not in yet has nothing to apply.
      messenger.sync(conversationId).catch(() => undefined);
    }
  });
  return <MessagingContext value={messaging}>{children}</MessagingContext>;
}

/** The messenger, once it is ready (or showing the new recovery phrase). */
export function messengerOf(messaging: Messaging): Messenger | undefined {
  return messaging.status === "ready" || messaging.status === "recovery-phrase"
    ? messaging.messenger
    : undefined;
}

/** Messaging; only inside a {@link MessagingProvider}. */
export function useMessaging(): Messaging {
  const messaging = use(MessagingContext);
  if (messaging === undefined) {
    throw new Error("useMessaging needs a MessagingProvider");
  }
  return messaging;
}
