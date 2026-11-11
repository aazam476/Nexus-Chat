/**
 * The composer's state: the draft, and whether it replies to or edits a
 * message. Sending turns it into the right payload.
 */
import { useState } from "react";

import { payloadLimits, type Payload } from "../../messaging/payload";
import type { TimelineItem } from "../../messaging/timeline";

/** What the next send does. */
export type ComposerMode =
  { kind: "new" } | { kind: "reply"; item: TimelineItem } | { kind: "edit"; item: TimelineItem };

/** The composer's state and actions. */
export interface Composer {
  draft: string;
  setDraft: (text: string) => void;
  mode: ComposerMode;
  replyTo: (item: TimelineItem) => void;
  edit: (item: TimelineItem) => void;
  cancel: () => void;
  tooLong: boolean;
  /** The payload to send, or undefined when there is nothing to send. */
  payload: () => Payload | undefined;
  /** Clears the composer after a send went out. */
  sent: () => void;
}

/** Keeps the draft and mode of a conversation's composer. */
export function useComposer(): Composer {
  const [draft, setDraft] = useState("");
  const [mode, setMode] = useState<ComposerMode>({ kind: "new" });
  const text = draft.trim();
  const tooLong = draft.length > payloadLimits.textLength;
  return {
    draft,
    setDraft,
    mode,
    replyTo: (item) => {
      setMode({ kind: "reply", item });
    },
    edit: (item) => {
      setMode({ kind: "edit", item });
      setDraft(item.text);
    },
    cancel: () => {
      setMode({ kind: "new" });
      setDraft("");
    },
    tooLong,
    payload: () => {
      if (text === "" || tooLong) {
        return undefined;
      }
      if (mode.kind === "edit") {
        return { kind: "edit", target: mode.item.id, text };
      }
      return mode.kind === "reply" ? { kind: "text", text, replyTo: mode.item.id } : { kind: "text", text };
    },
    sent: () => {
      setMode({ kind: "new" });
      setDraft("");
    },
  };
}
