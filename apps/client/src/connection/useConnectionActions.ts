/**
 * Everything that changes the connection: choosing or forgetting an
 * instance, and signing in and out.
 */
import { createApiClient, logout, type Client, type CurrentUser } from "@nexus-chat/api-client";
import { useCallback } from "react";

import { storageKeys, type SecureStorage } from "../storage/storage";
import { loadInstance } from "./loadConnection";
import type { ConnectionValues } from "./useConnectionState";

/** The connection's actions. */
export interface ConnectionActions {
  /** Connects to another instance; false when it does not answer. */
  chooseInstance: (address: string) => Promise<boolean>;
  /** Forgets the instance (and the session on it). */
  forgetInstance: () => Promise<void>;
  /** Stores a new session after a login, registration or provider sign-in. */
  signedIn: (token: string, user: CurrentUser) => Promise<void>;
  /** Logs out on the server and forgets the session. */
  signOut: () => Promise<void>;
  /** Reloads the instance's public state, for example after setup. */
  refreshInstance: () => Promise<void>;
  /** Replaces the user, after a profile or status change. */
  setUser: (user: CurrentUser) => void;
}

interface Dependencies {
  storage: SecureStorage;
  client: Client | undefined;
  values: ConnectionValues;
  setValues: (change: (values: ConnectionValues) => ConnectionValues) => void;
}

/** The actions, bound to the connection's state. */
export function useConnectionActions(dependencies: Dependencies): ConnectionActions {
  const session = useSessionActions(dependencies);
  const instance = useInstanceActions(dependencies, session.signOut);
  return { ...session, ...instance };
}

/** Signing in and out, and updating the user. */
function useSessionActions({
  storage,
  client,
  values,
  setValues,
}: Dependencies): Pick<ConnectionActions, "signedIn" | "signOut" | "setUser"> {
  const signedIn = useCallback(
    async (sessionToken: string, user: CurrentUser) => {
      await storage.set(storageKeys.session, sessionToken);
      setValues((current) => ({ ...current, user, sessionToken }));
    },
    [storage, setValues],
  );
  const signOut = useCallback(async () => {
    if (client !== undefined && values.sessionToken !== undefined) {
      await logout({ client }).catch(() => undefined); // offline: still forget it here
    }
    await storage.remove(storageKeys.session);
    setValues((current) => ({ ...current, user: undefined, sessionToken: undefined }));
  }, [client, values.sessionToken, storage, setValues]);
  const setUser = useCallback(
    (user: CurrentUser) => {
      setValues((current) => ({ ...current, user }));
    },
    [setValues],
  );
  return { signedIn, signOut, setUser };
}

/** Choosing, forgetting and reloading the instance. */
function useInstanceActions(
  { storage, client, setValues }: Dependencies,
  signOut: () => Promise<void>,
): Pick<ConnectionActions, "chooseInstance" | "forgetInstance" | "refreshInstance"> {
  const chooseInstance = useCallback(
    async (address: string) => {
      const instance = await loadInstance(createApiClient({ baseUrl: address }));
      if (instance !== undefined) {
        await storage.set(storageKeys.instance, address);
        setValues(() => ({ address, instance, user: undefined, sessionToken: undefined }));
      }
      return instance !== undefined;
    },
    [storage, setValues],
  );
  const forgetInstance = useCallback(async () => {
    await signOut();
    await storage.remove(storageKeys.instance);
    setValues(() => ({ address: undefined, instance: undefined, user: undefined, sessionToken: undefined }));
  }, [signOut, storage, setValues]);
  const refreshInstance = useCallback(async () => {
    const instance = client === undefined ? undefined : await loadInstance(client);
    setValues((current) => ({ ...current, instance }));
  }, [client, setValues]);
  return { chooseInstance, forgetInstance, refreshInstance };
}
