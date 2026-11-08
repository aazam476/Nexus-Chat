/**
 * Passwordless login with a passkey (spec §4.1). The browser asks the user
 * to pick a passkey; the server checks it and starts a session.
 */
import { beginPasskeyLogin, finishPasskeyLogin } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { deviceName } from "./deviceName";
import type { Message } from "./errors";
import { getAssertion, passkeysSupported } from "./webauthn";

/** State and action of the passkey button. */
export interface PasskeyLogin {
  supported: boolean;
  busy: boolean;
  error: Message | undefined;
  logIn: () => Promise<void>;
}

/** Runs a passkey login against the current instance. */
export function usePasskeyLogin(): PasskeyLogin {
  const connection = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Message | undefined>(undefined);
  const logIn = async (): Promise<void> => {
    setBusy(true);
    setError(undefined);
    try {
      const begun = await beginPasskeyLogin({ client: connection.client });
      if (begun.data === undefined) {
        setError({ key: "login.passkeyUnavailable" });
        return;
      }
      const credential = await getAssertion(begun.data.options);
      const body = { ceremonyId: begun.data.ceremonyId, credential, deviceName: deviceName() };
      const finished = await finishPasskeyLogin({ client: connection.client, body });
      if (finished.data === undefined) {
        setError({ key: "login.passkeyFailed" });
        return;
      }
      await connection.signedIn(finished.data.token, finished.data.user);
    } catch {
      setError({ key: "login.passkeyFailed" }); // cancelled, or the authenticator refused
    } finally {
      setBusy(false);
    }
  };
  return { supported: passkeysSupported(), busy, error, logIn };
}
