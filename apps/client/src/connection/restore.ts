/**
 * Reopening the app where it was: the saved instance (or, on web, the one
 * serving the page) and the saved session, if it still works.
 */
import { createApiClient } from "@nexus-chat/api-client";

import { defaultAddress } from "../instance/defaultAddress";
import { storageKeys, type SecureStorage } from "../storage/storage";
import { loadInstance, loadUser } from "./loadConnection";
import type { ConnectionValues } from "./useConnectionState";

/** The connection saved by an earlier run. */
export async function restore(storage: SecureStorage): Promise<ConnectionValues> {
  const address = (await storage.get(storageKeys.instance)) ?? defaultAddress();
  if (address === undefined) {
    return { address, instance: undefined, user: undefined, sessionToken: undefined };
  }
  const token = await storage.get(storageKeys.session);
  const client = createApiClient({ baseUrl: address, token: () => token });
  const instance = await loadInstance(client);
  const user = token === undefined || instance === undefined ? undefined : await loadUser(client);
  if (user === undefined && token !== undefined) {
    await storage.remove(storageKeys.session); // expired or revoked
  }
  return { address, instance, user, sessionToken: user === undefined ? undefined : token };
}
