/**
 * Finishing a provider sign-in on web, on the page the provider returns
 * to. The flow ID saved before leaving and the code in the address must
 * both be present (ADR 0016).
 */
import type { MfaChallenge } from "@nexus-chat/api-client";
import { useEffect, useState } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { storageKeys } from "../storage/storage";
import type { Message } from "./errors";
import { finishProviderSignIn, parseReturn } from "./providerReturn";

/** Where finishing the sign-in got to. */
export type ReturnState =
  | { kind: "working" }
  | { kind: "secondFactor"; challenge: MfaChallenge }
  | { kind: "failed"; message: Message };

/** Finishes the sign-in described by `fragment` (the return address's `#…`). */
export function useProviderReturn(fragment: string): ReturnState {
  const connection = useConnection();
  const [state, setState] = useState<ReturnState>({ kind: "working" });
  const { client, storage, signedIn } = connection;
  useEffect(() => {
    if (client === undefined) {
      return;
    }
    void (async () => {
      const flowId = await storage.get(storageKeys.providerFlow);
      await storage.remove(storageKeys.providerFlow);
      if (flowId === undefined) {
        setState({ kind: "failed", message: { key: "login.providerFailed" } });
        return;
      }
      const outcome = await finishProviderSignIn(client, flowId, parseReturn(fragment));
      if (outcome.kind === "signedIn") {
        await signedIn(outcome.result.token, outcome.result.user);
      } else if (outcome.kind === "secondFactor") {
        setState({ kind: "secondFactor", challenge: outcome.challenge });
      } else {
        setState(outcome);
      }
    })();
  }, [client, storage, signedIn, fragment]);
  return state;
}
