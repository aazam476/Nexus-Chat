/**
 * The app's connection: which instance it talks to, that instance's public
 * state, and who is logged in. Everything is kept in secure storage, so the
 * app opens where the user left it.
 */
import { createApiClient, type Client, type CurrentUser } from "@nexus-chat/api-client";
import { useEffect, useMemo, useState } from "react";

import type { SecureStorage } from "../storage/storage";
import type { InstanceState } from "./loadConnection";
import { restore } from "./restore";
import { useConnectionActions, type ConnectionActions } from "./useConnectionActions";

/** Everything the screens know about the connection. */
export interface Connection extends ConnectionActions {
  /** Where the instance, session and sign-ins in progress are kept. */
  storage: SecureStorage;
  loading: boolean;
  address: string | undefined;
  client: Client | undefined;
  instance: InstanceState | undefined;
  user: CurrentUser | undefined;
  /** The session token, for the gateway; undefined when logged out. */
  sessionToken: string | undefined;
}

/** The connection's state; the actions change it. */
export interface ConnectionValues {
  address: string | undefined;
  instance: InstanceState | undefined;
  user: CurrentUser | undefined;
  sessionToken: string | undefined;
}

const nothing: ConnectionValues = {
  address: undefined,
  instance: undefined,
  user: undefined,
  sessionToken: undefined,
};

/** Builds the connection on top of `storage`. */
export function useConnectionState(storage: SecureStorage): Connection {
  const [loading, setLoading] = useState(true);
  const [values, setValues] = useState<ConnectionValues>(nothing);
  const { address, sessionToken } = values;
  const client = useMemo(
    () =>
      address === undefined ? undefined : createApiClient({ baseUrl: address, token: () => sessionToken }),
    [address, sessionToken],
  );
  useEffect(() => {
    void restore(storage).then((restored) => {
      setValues(restored);
      setLoading(false);
    });
  }, [storage]);
  const actions = useConnectionActions({ storage, client, values, setValues });
  return { storage, loading, client, ...values, ...actions };
}
