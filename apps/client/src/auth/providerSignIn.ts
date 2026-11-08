/**
 * Starting a provider sign-in on iOS and Android: the provider opens in an
 * in-app browser session that returns to the `nexuschat://` scheme, and the
 * sign-in finishes without leaving the app.
 */
import { beginProviderSignIn, type Client } from "@nexus-chat/api-client";
import * as WebBrowser from "expo-web-browser";

import type { SecureStorage } from "../storage/storage";
import { appReturnAddress, finishProviderSignIn, parseReturn, type ProviderOutcome } from "./providerReturn";

/** Signs in with `providerId`; the result arrives when the browser closes. */
export async function startProviderSignIn(
  client: Client,
  providerId: string,
  _storage: SecureStorage,
): Promise<ProviderOutcome | undefined> {
  const begun = await beginProviderSignIn({
    client,
    path: { providerId },
    body: { returnTo: appReturnAddress },
  });
  if (begun.data === undefined) {
    return { kind: "failed", message: { key: "login.providerFailed" } };
  }
  const session = await WebBrowser.openAuthSessionAsync(begun.data.authorizationUrl, appReturnAddress);
  if (session.type !== "success") {
    return undefined; // the user closed the browser
  }
  const fragment = session.url.split("#")[1] ?? "";
  return finishProviderSignIn(client, begun.data.flowId, parseReturn(fragment));
}
