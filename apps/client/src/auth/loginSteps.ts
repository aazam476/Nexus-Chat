/**
 * The API calls of the password login, as plain functions: each returns
 * what the screen should do next.
 */
import {
  completeLoginMfa,
  login,
  type Client,
  type LoginResult,
  type MfaMethod,
} from "@nexus-chat/api-client";

import { deviceName } from "./deviceName";
import { messageFor, type Message } from "./errors";

/** What happened after a login step. */
export type StepOutcome =
  | { kind: "signedIn"; result: LoginResult }
  | { kind: "secondFactor"; challenge: string; methods: MfaMethod[] }
  | { kind: "failed"; message: Message };

const passwordErrors = {
  invalid_credentials: "login.invalidCredentials",
  rate_limited: "login.rateLimited",
  login_delayed: "login.delayed",
};

/** Checks the password; may ask for a second factor. */
export async function passwordStep(
  client: Client | undefined,
  loginName: string,
  password: string,
): Promise<StepOutcome> {
  const result = await login({
    client,
    body: { login: loginName.trim(), password, deviceName: deviceName() },
  });
  if (result.data === undefined) {
    const message = messageFor(result.error, passwordErrors);
    if (message.key === "login.delayed") {
      message.values = { seconds: Number(result.response?.headers.get("Retry-After") ?? "60") };
    }
    return { kind: "failed", message };
  }
  if ("challengeToken" in result.data) {
    return { kind: "secondFactor", challenge: result.data.challengeToken, methods: result.data.methods };
  }
  return { kind: "signedIn", result: result.data };
}

/** Checks the second factor. The challenge allows one attempt, so a wrong code ends the login. */
export async function secondFactorStep(
  client: Client | undefined,
  challenge: string,
  method: MfaMethod,
  code: string,
): Promise<StepOutcome> {
  const body = { challengeToken: challenge, method, code: code.trim(), deviceName: deviceName() };
  const result = await completeLoginMfa({ client, body });
  if (result.data === undefined) {
    return { kind: "failed", message: { key: "mfa.invalid" } };
  }
  return { kind: "signedIn", result: result.data };
}
