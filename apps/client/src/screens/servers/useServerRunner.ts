/** Runs a server action at a time: busy while it runs, and its error in words. */
import type { Client } from "@nexus-chat/api-client";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { useConnection } from "../../connection/ConnectionContext";
import { serverErrorText, type ServerResult } from "../../servers/serverActions";

/** An action on the instance; undefined when the user cancelled it. */
export type ServerAction = (client: Client) => Promise<ServerResult | undefined>;

/** What a management screen needs to run actions. */
export interface ServerRunner {
  busy: boolean;
  error: string | undefined;
  /** Runs `action`, then `then` with its result's ID if it worked. */
  run: (action: ServerAction, then?: (id: string) => void) => void;
}

/** Keeps the state of the screen's actions. */
export function useServerRunner(): ServerRunner {
  const { t } = useTranslation();
  const { client } = useConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const run = (action: ServerAction, then?: (id: string) => void): void => {
    if (client === undefined || busy) {
      return;
    }
    setBusy(true);
    setError(undefined);
    void action(client).then((result) => {
      setBusy(false);
      if (result?.ok === false) {
        setError(serverErrorText(result.code, t));
      } else if (result !== undefined) {
        then?.(result.id);
      }
    });
  };
  return { busy, error, run };
}
