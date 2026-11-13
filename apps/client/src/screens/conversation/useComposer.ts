/**
 * The composer's state: the draft, and whether it replies to or edits a
 * message. Sending turns it into the right payload.
 */
import { useState } from "react";

import type { PickedFile } from "../../messaging/attachmentTransfer";
import { payloadLimits, type AttachmentReference, type Payload } from "../../messaging/payload";
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
  /** Files to send with the next message, not uploaded yet. */
  files: PickedFile[];
  attach: (file: PickedFile) => void;
  detach: (index: number) => void;
  /** Sends a photo's original instead of its compressed copy, or not. */
  setOriginal: (index: number, original: boolean) => void;
  /**
   * The payload to send, with the uploaded files' references, or
   * undefined when there is nothing to send.
   */
  payload: (attachments?: AttachmentReference[]) => Payload | undefined;
  /** Clears the composer after a send went out. */
  sent: () => void;
}

/** The payload for the composer's state, or undefined when there is nothing to send. */
function payloadOf(
  state: { text: string; mode: ComposerMode; files: PickedFile[] },
  attachments: AttachmentReference[],
): Payload | undefined {
  const { text, mode, files } = state;
  if (mode.kind === "edit") {
    return text === "" ? undefined : { kind: "edit", target: mode.item.id, text };
  }
  if (text === "" && files.length === 0) {
    return undefined;
  }
  return {
    kind: "text",
    text,
    ...(mode.kind === "reply" ? { replyTo: mode.item.id } : {}),
    ...(attachments.length === 0 ? {} : { attachments }),
  };
}

/** The files waiting to go with the next message. */
function usePendingFiles(): Pick<Composer, "files" | "attach" | "detach" | "setOriginal"> & {
  clear: () => void;
} {
  const [files, setFiles] = useState<PickedFile[]>([]);
  return {
    files,
    attach: (file) => {
      setFiles((current) => (current.length < payloadLimits.attachments ? [...current, file] : current));
    },
    detach: (index) => {
      setFiles((current) => current.filter((_, at) => at !== index));
    },
    setOriginal: (index, original) => {
      setFiles((current) =>
        current.map((file, at) => (at === index ? { ...file, sendOriginal: original } : file)),
      );
    },
    clear: () => {
      setFiles([]);
    },
  };
}

/** Keeps the draft, mode and pending files of a conversation's composer. */
export function useComposer(): Composer {
  const [draft, setDraft] = useState("");
  const [mode, setMode] = useState<ComposerMode>({ kind: "new" });
  const { clear, ...pending } = usePendingFiles();
  const text = draft.trim();
  const tooLong = draft.length > payloadLimits.textLength;
  const reset = (): void => {
    setMode({ kind: "new" });
    setDraft("");
    clear();
  };
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
      clear();
    },
    cancel: reset,
    tooLong,
    ...pending,
    payload: (attachments = []) =>
      tooLong ? undefined : payloadOf({ text, mode, files: pending.files }, attachments),
    sent: reset,
  };
}
