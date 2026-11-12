/**
 * Sending what the composer holds: its files are encrypted and uploaded
 * first, then the message goes out with their references. One send at a
 * time; a failure keeps the draft and files so the user can try again.
 */
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { AttachmentTransferError } from "../../messaging/attachmentTransfer";
import { fileSize } from "../../messaging/fileSize";
import type { Payload } from "../../messaging/payload";
import type { ConversationAttachments } from "../../messaging/useAttachments";
import type { Composer } from "./useComposer";

/** Submitting, and why the last attempt failed. */
export interface Submit {
  submit: () => void;
  busy: boolean;
  error: string | undefined;
}

/** Sends the composer's message with `send`, uploading its files through `attachments`. */
export function useSubmit(
  composer: Composer,
  send: (payload: Payload) => Promise<unknown>,
  attachments: ConversationAttachments,
): Submit {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const explain = (cause: unknown): string =>
    cause instanceof AttachmentTransferError && cause.code === "too_large"
      ? t("conversation.attachmentTooLarge", { limit: fileSize(attachments.maxBytes, t) })
      : cause instanceof AttachmentTransferError
        ? t("conversation.attachmentFailed")
        : t("conversation.sendFailed");
  const submit = (): void => {
    if (composer.payload() === undefined || busy) {
      return;
    }
    setBusy(true);
    setError(undefined);
    void (async () => {
      const references = await attachments.upload(composer.files);
      const payload = composer.payload(references);
      if (payload !== undefined) {
        await send(payload);
      }
    })()
      .then(composer.sent, (cause: unknown) => {
        setError(explain(cause));
      })
      .finally(() => {
        setBusy(false);
      });
  };
  return { submit, busy, error };
}
