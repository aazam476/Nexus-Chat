/**
 * Starting a provider sign-in on web: the whole page goes to the provider
 * and comes back to `/oidc-return`. The flow ID must survive that trip, so
 * it is kept in the encrypted storage; it is useless without the code the
 * provider returns.
 */
import { beginProviderSignIn, type Client } from "@nexus-chat/api-client";

import { storageKeys, type SecureStorage } from "../storage/storage";
import { returnPath, type ProviderOutcome } from "./providerReturn";

/** Sends the browser to the provider; nothing is returned, as the page unloads. */
export async function startProviderSignIn(
  client: Client,
  providerId: string,
  storage: SecureStorage,
): Promise<ProviderOutcome | undefined> {
  const begun = await beginProviderSignIn({ client, path: { providerId }, body: { returnTo: returnPath } });
  if (begun.data === undefined) {
    return { kind: "failed", message: { key: "login.providerFailed" } };
  }
  await storage.set(storageKeys.providerFlow, begun.data.flowId);
  window.location.assign(begun.data.authorizationUrl);
  return undefined;
}
