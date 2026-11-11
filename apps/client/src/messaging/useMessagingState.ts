/**
 * Messaging for the signed-in user, as React state: it starts the
 * messenger once a user is logged in, and says what the user has to do
 * first (write down a new recovery phrase, or restore on a new device).
 */
import { listSessions, type Client } from "@nexus-chat/api-client";
import { useCallback, useEffect, useRef, useState } from "react";

import type { Connection } from "../connection/useConnectionState";
import { hostOf } from "../instance/address";
import { scopedDatabase, type LocalDatabase } from "../storage/localDatabase";
import type { MessagingApi } from "./engine/api";
import { fromBase64 } from "./engine/bytes";
import {
  confirmRecoveryPhrase,
  restoreMessenger,
  startMessenger,
  type Messenger,
  type MessengerSetup,
} from "./messenger";

/** Where the device's database and the server calls come from; tests pass fakes. */
export interface MessagingEnvironment {
  openDatabase: () => Promise<LocalDatabase>;
  createApi: (client: Client) => MessagingApi;
}

/** Where messaging stands for the signed-in user. */
export type MessagingStatus =
  | { status: "off" }
  | { status: "starting" }
  | { status: "failed" }
  | { status: "needs-restore" }
  | { status: "recovery-phrase"; phrase: string; messenger: Messenger }
  | { status: "ready"; messenger: Messenger };

/** The state and what the user can do about it. */
export type Messaging = MessagingStatus & {
  /** Restores from the recovery phrase; resolves false when it does not open the backup. */
  restore: (phrase: string) => Promise<boolean>;
  /** The user wrote the phrase down. */
  confirmPhrase: () => Promise<void>;
  retry: () => void;
};

/** This device's setup for the signed-in user, or undefined when logged out. */
async function setupFor(
  connection: Connection,
  environment: MessagingEnvironment,
): Promise<MessengerSetup | undefined> {
  const { client, user, instance, address } = connection;
  if (client === undefined || user === undefined || instance === undefined || address === undefined) {
    return undefined;
  }
  const current = (await listSessions({ client })).data?.find((session) => session.current);
  if (current === undefined) {
    throw new Error("the server did not report this session");
  }
  const domain = instance.info.domain ?? hostOf(address);
  return {
    api: environment.createApi(client),
    database: scopedDatabase(await environment.openDatabase(), `${user.id}@${domain}`),
    user: { id: user.id, username: user.username },
    where: { domain, deviceId: current.id },
    instanceKey: fromBase64(instance.info.signingKey),
  };
}

/** Sets messaging up and reports each step through `apply`. */
async function begin(
  connection: Connection,
  environment: MessagingEnvironment,
  apply: (next: MessagingStatus, setup?: MessengerSetup) => void,
): Promise<void> {
  const setup = await setupFor(connection, environment);
  if (setup === undefined) {
    apply({ status: "off" });
    return;
  }
  apply({ status: "starting" }, setup);
  const start = await startMessenger(setup);
  if (start.status === "needs-restore") {
    apply(start);
    return;
  }
  const { messenger, recoveryPhrase } = start;
  apply(
    recoveryPhrase === undefined
      ? { status: "ready", messenger }
      : { status: "recovery-phrase", phrase: recoveryPhrase, messenger },
  );
}

/** Starts messaging for whoever is signed in on `connection`. */
export function useMessagingState(connection: Connection, environment: MessagingEnvironment): Messaging {
  const [state, setState] = useState<MessagingStatus>({ status: "off" });
  const [setup, setSetup] = useState<MessengerSetup>();
  const [attempt, setAttempt] = useState(0);
  const userId = connection.user?.id;
  const client = connection.client;
  // The latest connection and environment, read when (re)starting.
  const latestRef = useRef({ connection, environment });
  useEffect(() => {
    latestRef.current = { connection, environment };
  });
  useEffect(() => {
    let current = true;
    const apply = (next: MessagingStatus, found?: MessengerSetup): void => {
      if (current) {
        setState(next);
        if (found !== undefined) {
          setSetup(found);
        }
      }
    };
    begin(latestRef.current.connection, latestRef.current.environment, apply).catch(() => {
      apply({ status: "failed" });
    });
    return () => {
      current = false;
    };
    // Starts again for another user or instance, or when asked to retry.
  }, [userId, client, attempt]);
  const actions = useActions(setup, state, setState);
  return {
    ...state,
    ...actions,
    retry: () => {
      setAttempt((count) => count + 1);
    },
  };
}

/** Restoring and confirming the phrase. */
function useActions(
  setup: MessengerSetup | undefined,
  state: MessagingStatus,
  setState: (next: MessagingStatus) => void,
): Pick<Messaging, "restore" | "confirmPhrase"> {
  const restore = useCallback(
    async (phrase: string): Promise<boolean> => {
      if (setup === undefined) {
        return false;
      }
      try {
        setState({ status: "ready", messenger: await restoreMessenger(setup, phrase) });
        return true;
      } catch {
        return false;
      }
    },
    [setup, setState],
  );
  const confirmPhrase = useCallback(async (): Promise<void> => {
    if (setup !== undefined && state.status === "recovery-phrase") {
      await confirmRecoveryPhrase(setup.database);
      setState({ status: "ready", messenger: state.messenger });
    }
  }, [setup, state, setState]);
  return { restore, confirmPhrase };
}
