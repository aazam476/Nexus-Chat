/**
 * The sign-in providers the instance offers (spec §4.1), and starting a
 * sign-in with one of them.
 */
import { listSignInProviders, type SignInProvider } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import type { Message } from "./errors";
import type { ProviderOutcome } from "./providerReturn";
import { startProviderSignIn } from "./providerSignIn";

/** State and action of the provider buttons. */
export interface Providers {
  providers: SignInProvider[];
  error: Message | undefined;
  /** Starts a sign-in; on native, the outcome once the browser closes. */
  signIn: (providerId: string) => Promise<ProviderOutcome | undefined>;
}

/** Lists the instance's providers. */
export function useProviders(): Providers {
  const connection = useConnection();
  const [providers, setProviders] = useState<SignInProvider[]>([]);
  const [error, setError] = useState<Message | undefined>(undefined);
  const { client, storage } = connection;
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    void listSignInProviders({ client }).then((result) => {
      setProviders(result.data ?? []);
    });
  }, [client]);
  const signIn = async (providerId: string): Promise<ProviderOutcome | undefined> => {
    if (client === undefined) {
      return undefined;
    }
    setError(undefined);
    const outcome = await startProviderSignIn(client, providerId, storage);
    if (outcome?.kind === "failed") {
      setError(outcome.message);
    } else if (outcome?.kind === "signedIn") {
      await connection.signedIn(outcome.result.token, outcome.result.user);
    }
    return outcome;
  };
  return { providers, error, signIn };
}
