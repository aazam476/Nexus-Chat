import { createContext, use, type ReactNode } from "react";

import type { SecureStorage } from "../storage/storage";
import { useConnectionState, type Connection } from "./useConnectionState";

/** The connection; tests provide a fake one directly. */
export const ConnectionContext = createContext<Connection | undefined>(undefined);

/** Props of {@link ConnectionProvider}. */
export interface ConnectionProviderProps {
  storage: SecureStorage;
  children: ReactNode;
}

/** Makes the connection available to every screen. */
export function ConnectionProvider({ storage, children }: ConnectionProviderProps): ReactNode {
  const connection = useConnectionState(storage);
  return <ConnectionContext value={connection}>{children}</ConnectionContext>;
}

/** The connection; only inside a {@link ConnectionProvider}. */
export function useConnection(): Connection {
  const connection = use(ConnectionContext);
  if (connection === undefined) {
    throw new Error("useConnection needs a ConnectionProvider");
  }
  return connection;
}
