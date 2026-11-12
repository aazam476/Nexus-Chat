/**
 * Sending and fetching encrypted attachments (docs/e2ee-design.md §7). A
 * file is encrypted on this device with a fresh key, uploaded as
 * ciphertext, and described in the message by an AttachmentReference,
 * which carries the key inside the MLS-encrypted payload. Recipients
 * download the ciphertext, check its hash and decrypt it.
 */
import {
  decryptFile,
  decryptStream,
  encryptFile,
  encryptStream,
  oneShotLimit,
  type AttachmentSecrets,
} from "@nexus-chat/crypto";
import { downloadAttachment, uploadAttachment, type Client } from "@nexus-chat/api-client";

import { fromBase64, toBase64 } from "./engine/bytes";
import type { AttachmentReference } from "./payload";

/** Thrown when a file can't be sent or fetched; `code` says why. */
export class AttachmentTransferError extends Error {
  override name = "AttachmentTransferError";
  constructor(readonly code: "too_large" | "upload_failed" | "download_failed") {
    super(code);
  }
}

/** A file the user picked, read into memory. */
export interface PickedFile {
  name: string;
  mimeType: string;
  bytes: Uint8Array<ArrayBuffer>;
  /** For images, read before encryption (spec §4.5). */
  width?: number;
  height?: number;
  /** For images, a small preview made before encryption. */
  thumbnail?: { bytes: Uint8Array<ArrayBuffer>; mimeType: string; width: number; height: number };
}

/** Where to upload and how large a file may be. */
export interface Destination {
  client: Client;
  conversationId: string;
  /** The instance's attachment limit, in plaintext bytes. */
  maxBytes: number;
}

/** The bytes as a one-piece stream, for the stream functions. */
function once(bytes: Uint8Array): AsyncIterable<Uint8Array> {
  return {
    [Symbol.asyncIterator]: () => {
      let done = false;
      return {
        next: () => {
          const result: IteratorResult<Uint8Array> = done
            ? { done: true, value: undefined }
            : { done: false, value: bytes };
          done = true;
          return Promise.resolve(result);
        },
      };
    },
  };
}

async function concatenate(pieces: AsyncIterable<Uint8Array>): Promise<Uint8Array<ArrayBuffer>> {
  const parts: Uint8Array[] = [];
  for await (const piece of pieces) {
    parts.push(piece);
  }
  const out = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

/** Encrypts in one piece up to 64 MiB, and as a stream above. */
async function encrypt(
  bytes: Uint8Array<ArrayBuffer>,
): Promise<{ ciphertext: Uint8Array<ArrayBuffer>; secrets: AttachmentSecrets }> {
  if (bytes.length <= oneShotLimit) {
    return encryptFile(bytes);
  }
  const stream = await encryptStream(once(bytes));
  const ciphertext = await concatenate(stream.ciphertext);
  return { ciphertext, secrets: stream.secrets() };
}

/** Encrypts and uploads bytes; returns the stored file's ID and its secrets. */
async function upload(
  destination: Destination,
  bytes: Uint8Array<ArrayBuffer>,
): Promise<{ id: string; secrets: AttachmentSecrets }> {
  const { ciphertext, secrets } = await encrypt(bytes);
  const { client, conversationId } = destination;
  const result = await uploadAttachment({
    client,
    path: { conversationId },
    query: { size: ciphertext.length },
    body: new Blob([ciphertext]),
  });
  if (result.data === undefined) {
    throw new AttachmentTransferError("upload_failed");
  }
  return { id: result.data.id, secrets };
}

function secretFields(
  secrets: AttachmentSecrets,
): Pick<AttachmentReference, "scheme" | "key" | "nonce" | "ciphertextSha256"> {
  return {
    scheme: secrets.scheme,
    key: toBase64(secrets.key),
    ...(secrets.nonce === undefined ? {} : { nonce: toBase64(secrets.nonce) }),
    ciphertextSha256: toBase64(secrets.ciphertextSha256),
  };
}

/** Encrypts, uploads and describes a picked file, with its thumbnail. */
export async function sendAttachment(
  destination: Destination,
  file: PickedFile,
): Promise<AttachmentReference> {
  if (file.bytes.length > destination.maxBytes) {
    throw new AttachmentTransferError("too_large");
  }
  const stored = await upload(destination, file.bytes);
  const reference: AttachmentReference = {
    id: stored.id,
    name: file.name,
    mimeType: file.mimeType,
    size: file.bytes.length,
    ...(file.width === undefined || file.height === undefined
      ? {}
      : { width: file.width, height: file.height }),
    ...secretFields(stored.secrets),
  };
  if (file.thumbnail === undefined) {
    return reference;
  }
  const { bytes, mimeType, width, height } = file.thumbnail;
  const thumbnail = await upload(destination, bytes);
  return {
    ...reference,
    thumbnail: { id: thumbnail.id, mimeType, width, height, ...secretFields(thumbnail.secrets) },
  };
}

/** Downloads, checks and decrypts a file or thumbnail the message refers to. */
export async function fetchAttachment(
  client: Client,
  conversationId: string,
  file: Pick<AttachmentReference, "id" | "scheme" | "key" | "nonce" | "ciphertextSha256">,
): Promise<Uint8Array<ArrayBuffer>> {
  const result = await downloadAttachment({
    client,
    path: { conversationId, attachmentId: file.id },
    parseAs: "arrayBuffer",
  });
  if (!(result.data instanceof ArrayBuffer)) {
    throw new AttachmentTransferError("download_failed");
  }
  const secrets: AttachmentSecrets = {
    scheme: file.scheme === "xchacha20poly1305-secretstream" ? file.scheme : "aes-256-gcm",
    key: fromBase64(file.key),
    ...(file.nonce === undefined ? {} : { nonce: fromBase64(file.nonce) }),
    ciphertextSha256: fromBase64(file.ciphertextSha256),
  };
  const ciphertext = new Uint8Array(result.data);
  return secrets.scheme === "aes-256-gcm"
    ? decryptFile(ciphertext, secrets)
    : concatenate(decryptStream(once(ciphertext), secrets));
}
