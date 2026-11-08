/**
 * The state every settings form shares while it saves a change: busy, the
 * error if it failed, and the confirmation if it worked.
 */
import { useState } from "react";

import type { FieldMessages, Message } from "../auth/errors";

/** What a change reports when it finishes. */
export interface ChangeOutcome {
  error?: Message;
  fields?: FieldMessages;
  notice?: Message;
}

/** A form's change state and the function that runs a change. */
export interface Change {
  busy: boolean;
  error: Message | undefined;
  fields: FieldMessages;
  notice: Message | undefined;
  /** Runs `change`, clearing the previous outcome first. */
  run: (change: () => Promise<ChangeOutcome>) => Promise<ChangeOutcome>;
}

/** Keeps the state of one form's changes. */
export function useChange(): Change {
  const [busy, setBusy] = useState(false);
  const [outcome, setOutcome] = useState<ChangeOutcome>({});
  const run = async (change: () => Promise<ChangeOutcome>): Promise<ChangeOutcome> => {
    setBusy(true);
    setOutcome({});
    try {
      const finished = await change();
      setOutcome(finished);
      return finished;
    } finally {
      setBusy(false);
    }
  };
  return { busy, error: outcome.error, fields: outcome.fields ?? {}, notice: outcome.notice, run };
}
