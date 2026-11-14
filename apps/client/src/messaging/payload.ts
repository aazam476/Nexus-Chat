/**
 * What travels inside an MLS application message (and its archive copy):
 * one JSON object with a version and a kind. Edits, deletions, reactions
 * and replies are ordinary messages that name the message they act on
 * (docs/e2ee-design.md §4), so every device builds the same view from the
 * same stream. Unknown kinds and fields are ignored, so newer clients can
 * add them; anything malformed is dropped rather than shown.
 */

/** A file sent with a message; the secrets come from packages/crypto attachments. */
export interface AttachmentReference {
  /** The upload's ID on the instance. */
  id: string;
  name: string;
  mimeType: string;
  size: number;
  width?: number;
  height?: number;
  /** "aes-256-gcm" or "xchacha20poly1305-secretstream". */
  scheme: string;
  /** Base64 key, nonce (one-shot only) and ciphertext SHA-256. */
  key: string;
  nonce?: string;
  ciphertextSha256: string;
  /** The encrypted thumbnail, for images and videos. */
  thumbnail?: Omit<AttachmentReference, "thumbnail" | "name" | "size">;
}

/** A link preview the sender's device made (spec §4.5). */
export interface LinkPreview {
  url: string;
  title?: string;
  description?: string;
  image?: AttachmentReference;
}

/** Every kind of payload. */
export type Payload =
  | {
      kind: "text";
      text: string;
      replyTo?: string;
      attachments?: AttachmentReference[];
      preview?: LinkPreview;
    }
  | { kind: "edit"; target: string; text: string }
  | { kind: "delete"; target: string }
  | { kind: "reaction"; target: string; emoji: string; added: boolean };

/** Limits that keep one message reasonable to show. */
export const payloadLimits = { textLength: 4000, attachments: 10, emojiLength: 64 } as const;

const version = 1;

/** A payload with the ID its sender gave it, which edits, reactions and replies refer to. */
export interface Envelope {
  id: string;
  payload: Payload;
  /**
   * In a server channel, the channel. It is inside the encryption, so the
   * server's tag on the message can be checked against it.
   */
  channel?: string;
}

/** The bytes to encrypt. */
export function encodePayload(envelope: Envelope): Uint8Array {
  const channel = envelope.channel === undefined ? {} : { channel: envelope.channel };
  return new TextEncoder().encode(
    JSON.stringify({ v: version, id: envelope.id, ...channel, ...envelope.payload }),
  );
}

type Fields = Record<string, unknown>;

const isText = (value: unknown, limit: number): value is string =>
  typeof value === "string" && value.length > 0 && value.length <= limit;
const isId = (value: unknown): value is string => typeof value === "string" && /^[0-9a-f-]{36}$/.test(value);

const isString = (value: unknown): value is string => typeof value === "string";
const isCount = (value: unknown): value is number =>
  typeof value === "number" && Number.isInteger(value) && value >= 0;

/** A thumbnail or attachment with every field this version needs, of the right type. */
function isFile(value: unknown, named: boolean): boolean {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const file = value as Fields;
  const required = [file.id, file.mimeType, file.scheme, file.key, file.ciphertextSha256].every(isString);
  const optional = [file.nonce].every((field) => field === undefined || isString(field));
  const dimensions = [file.width, file.height].every((field) => field === undefined || isCount(field));
  const naming = !named || (isString(file.name) && isCount(file.size));
  return required && optional && dimensions && naming;
}

function isAttachment(value: unknown): value is AttachmentReference {
  return (
    isFile(value, true) &&
    ((value as Fields).thumbnail === undefined || isFile((value as Fields).thumbnail, false))
  );
}

function isPreview(value: unknown): value is LinkPreview {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const preview = value as Fields;
  return (
    isString(preview.url) &&
    [preview.title, preview.description].every((field) => field === undefined || isString(field)) &&
    (preview.image === undefined || isAttachment(preview.image))
  );
}

function textPayload(fields: Fields): Payload | undefined {
  const { attachments, preview, replyTo } = fields;
  const hasText = isText(fields.text, payloadLimits.textLength);
  const validAttachments =
    attachments === undefined ||
    (Array.isArray(attachments) &&
      attachments.length <= payloadLimits.attachments &&
      attachments.every(isAttachment));
  if ((!hasText && attachments === undefined) || !validAttachments) {
    return undefined;
  }
  if ((replyTo !== undefined && !isId(replyTo)) || (preview !== undefined && !isPreview(preview))) {
    return undefined;
  }
  return {
    kind: "text",
    text: hasText ? (fields.text as string) : "",
    ...(isId(replyTo) ? { replyTo } : {}),
    ...(Array.isArray(attachments) ? { attachments: attachments } : {}),
    ...(isPreview(preview) ? { preview } : {}),
  };
}

/** Reads decrypted bytes, or undefined for anything this version can't show. */
export function decodePayload(bytes: Uint8Array): Envelope | undefined {
  const parsed = parse(bytes);
  if (parsed === undefined || !isId(parsed.id)) {
    return undefined;
  }
  const payload = payloadOf(parsed);
  if (payload === undefined) {
    return undefined;
  }
  return isId(parsed.channel)
    ? { id: parsed.id, payload, channel: parsed.channel }
    : { id: parsed.id, payload };
}

function parse(bytes: Uint8Array): Fields | undefined {
  let fields: unknown;
  try {
    fields = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    return undefined;
  }
  if (typeof fields !== "object" || fields === null || (fields as Fields).v !== version) {
    return undefined;
  }
  return fields as Fields;
}

function payloadOf(object: Fields): Payload | undefined {
  switch (object.kind) {
    case "text":
      return textPayload(object);
    case "edit":
      return isId(object.target) && isText(object.text, payloadLimits.textLength)
        ? { kind: "edit", target: object.target, text: object.text }
        : undefined;
    case "delete":
      return isId(object.target) ? { kind: "delete", target: object.target } : undefined;
    case "reaction":
      return isId(object.target) &&
        isText(object.emoji, payloadLimits.emojiLength) &&
        typeof object.added === "boolean"
        ? { kind: "reaction", target: object.target, emoji: object.emoji, added: object.added }
        : undefined;
    default:
      return undefined;
  }
}
