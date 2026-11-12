/** Uploading and opening one conversation's attachments. */
import { createContext, use } from "react";

import { useConnection } from "../connection/ConnectionContext";
import { fetchAttachment, sendAttachment, type PickedFile } from "./attachmentTransfer";
import type { AttachmentReference } from "./payload";

/** Uploads files, and fetches files the conversation's messages refer to. */
export interface ConversationAttachments {
  /** The instance's limit, in plaintext bytes. */
  maxBytes: number;
  upload: (files: PickedFile[]) => Promise<AttachmentReference[]>;
  download: (
    file: Pick<AttachmentReference, "id" | "scheme" | "key" | "nonce" | "ciphertextSha256">,
  ) => Promise<Uint8Array<ArrayBuffer>>;
}

/** Attachments of the conversation `conversationId`. */
export function useAttachments(conversationId: string): ConversationAttachments {
  const { client, instance } = useConnection();
  const maxBytes = instance?.info.limits.attachmentMaxBytes ?? 25 << 20;
  return {
    maxBytes,
    upload: async (files) => {
      if (client === undefined) {
        throw new Error("not connected");
      }
      const references: AttachmentReference[] = [];
      for (const file of files) {
        references.push(await sendAttachment({ client, conversationId, maxBytes }, file));
      }
      return references;
    },
    download: (file) =>
      client === undefined
        ? Promise.reject(new Error("not connected"))
        : fetchAttachment(client, conversationId, file),
  };
}

/** The open conversation's attachments, for the messages that show them. */
export const AttachmentsContext = createContext<ConversationAttachments | undefined>(undefined);

/** The open conversation's attachments; undefined outside a conversation. */
export function useConversationAttachments(): ConversationAttachments | undefined {
  return use(AttachmentsContext);
}
