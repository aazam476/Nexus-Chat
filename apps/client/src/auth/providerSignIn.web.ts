/**
 * Starting a provider sign-in on web. In a browser the whole page goes to
 * the provider and comes back to `/oidc-return`; the flow ID must survive
 * that trip, so it is kept in the encrypted storage (it is useless without
 * the code the provider returns). In the desktop app the provider opens in
 * a window of its own, and the shell hands back the return fragment.
 */
import { beginProviderSignIn, type Client } from "@nexus-chat/api-client";

import { storageKeys, type SecureStorage } from "../storage/storage";
import {
  appReturnAddress,
  finishProviderSignIn,
  parseReturn,
  returnPath,
  type ProviderOutcome,
} from "./providerReturn";

/** What the desktop shell's preload script offers the page. */
interface DesktopBridge {
  signInWithProvider(authorizationUrl: string): Promise<string | undefined>;
}

function desktopBridge(): DesktopBridge | undefined {
  return (window as { nexusDesktop?: DesktopBridge }).nexusDesktop;
}

/**
 * Signs in with `providerId`. In a browser nothing is returned, as the
 * page unloads; in the desktop app the outcome arrives when the window
 * closes, or undefined if the user closed it.
 */
export async function startProviderSignIn(
  client: Client,
  providerId: string,
  storage: SecureStorage,
): Promise<ProviderOutcome | undefined> {
  const desktop = desktopBridge();
  const begun = await beginProviderSignIn({
    client,
    path: { providerId },
    body: { returnTo: desktop === undefined ? returnPath : appReturnAddress },
  });
  if (begun.data === undefined) {
    return { kind: "failed", message: { key: "login.providerFailed" } };
  }
  if (desktop !== undefined) {
    const fragment = await desktop.signInWithProvider(begun.data.authorizationUrl);
    return fragment === undefined
      ? undefined
      : finishProviderSignIn(client, begun.data.flowId, parseReturn(fragment));
  }
  await storage.set(storageKeys.providerFlow, begun.data.flowId);
  window.location.assign(begun.data.authorizationUrl);
  return undefined;
}
