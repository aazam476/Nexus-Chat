/**
 * Running "search older messages" for one conversation or channel, with
 * its progress. A phone waits for Wi-Fi and charging unless the user says
 * to go ahead now.
 */
import { useState } from "react";

import { messengerOf, useMessaging } from "../messaging/MessagingContext";
import { goodTimeToBackfill } from "../platform/backfillConditions";

/** Where a backfill stands. */
export type BackfillState =
  | { kind: "idle" }
  | { kind: "running" }
  | { kind: "waiting"; indexed: number }
  | { kind: "done"; indexed: number }
  | { kind: "failed" };

/** The backfill's state, and starting (or resuming) it; `now` skips the wait for Wi-Fi and charging. */
export function useBackfill(id: string | undefined): { state: BackfillState; start: (now: boolean) => void } {
  const messenger = messengerOf(useMessaging());
  const [state, setState] = useState<BackfillState>({ kind: "idle" });
  const start = (now: boolean): void => {
    if (messenger === undefined || id === undefined) {
      return;
    }
    setState({ kind: "running" });
    messenger
      .searchOlder(id, now ? () => Promise.resolve(true) : goodTimeToBackfill)
      .then((progress) => {
        setState(
          progress.done
            ? { kind: "done", indexed: progress.indexed }
            : { kind: "waiting", indexed: progress.indexed },
        );
      })
      .catch(() => {
        setState({ kind: "failed" });
      });
  };
  return { state, start };
}
