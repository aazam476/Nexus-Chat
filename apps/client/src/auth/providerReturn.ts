/**
 * Reading the provider's answer and finishing the sign-in (ADR 0016). The
 * server sends the browser back to the app with `#flow=…&code=…` (or
 * `#error=…`) in the fragment, which never reaches any server's logs.
 */
import {
  completeProviderSignIn,
  type Client,
  type LoginResult,
  type MfaChallenge,
} from "@nexus-chat/api-client";

import { deviceName } from "./deviceName";
import { messageFor, type Message } from "./errors";

/** The path the provider returns to, on web. */
export const returnPath = "/oidc-return";

/** What came back from the provider. */
export interface ProviderReturn {
  code?: string;
  error?: string;
}

/** Reads the fragment of the return address. */
export function parseReturn(fragment: string): ProviderReturn {
  const values = new URLSearchParams(fragment.replace(/^#/, ""));
  const result: ProviderReturn = {};
  const code = values.get("code");
  const error = values.get("error");
  if (code !== null) {
    result.code = code;
  }
  if (error !== null) {
    result.error = error;
  }
  return result;
}

/** The outcome of finishing a sign-in. */
export type ProviderOutcome =
  | { kind: "signedIn"; result: LoginResult }
  | { kind: "secondFactor"; challenge: MfaChallenge }
  | { kind: "failed"; message: Message };

const completionErrors = {
  invalid_flow: "login.providerFailed",
  no_account: "login.providerNoAccount",
  registration_closed: "register.closed",
  invite_required: "register.inviteRequired",
  invalid_invite: "register.invalidInvite",
  email_required: "login.providerNoEmail",
  email_in_use: "login.providerEmailInUse",
};

/** Finishes the sign-in begun as `flowId` with what the provider returned. */
export async function finishProviderSignIn(
  client: Client,
  flowId: string,
  returned: ProviderReturn,
): Promise<ProviderOutcome> {
  if (returned.code === undefined) {
    return { kind: "failed", message: { key: "login.providerFailed" } };
  }
  const result = await completeProviderSignIn({
    client,
    body: { flowId, code: returned.code, deviceName: deviceName() },
  });
  if (result.data === undefined) {
    return { kind: "failed", message: messageFor(result.error, completionErrors) };
  }
  if ("challengeToken" in result.data) {
    return { kind: "secondFactor", challenge: result.data };
  }
  return { kind: "signedIn", result: result.data };
}
