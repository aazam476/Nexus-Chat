/**
 * Moderators' deletions (spec §4.7). A moderator deletes a channel message,
 * or a ban deletes what someone sent recently, by deleting the ciphertext
 * on the server; the instance then signs a directive for the channel's
 * group. Devices drop their decrypted copies for directives the instance
 * signed and no others, so nobody else can make a device forget messages.
 */
import { verifyDirective } from "@nexus-chat/crypto";

import { unindexMessages } from "../../search/indexer";
import { fromBase64 } from "./bytes";
import type { EngineContext } from "./context";
import type { DecryptedMessage } from "./sync";

/** A signed directive's contents, as the instance wrote them. */
export interface Directive {
  type: string;
  userId?: string;
  messageId?: string;
  channelId?: string;
  since?: string;
}

/** What a moderator removed: one message, or a user's messages since a time. */
export type Removal =
  { kind: "delete"; messageId: string; channelId: string } | { kind: "purge"; userId: string; since: string };

/** The directive in a delivered directive message, if the instance signed it. */
export async function readDirective(
  context: EngineContext,
  data: Uint8Array,
): Promise<Directive | undefined> {
  const signed = JSON.parse(new TextDecoder().decode(data)) as { directive?: string; signature?: string };
  const directive = fromBase64(signed.directive ?? "");
  if (!(await verifyDirective(context.instanceKey, directive, fromBase64(signed.signature ?? "")))) {
    return undefined;
  }
  return JSON.parse(new TextDecoder().decode(directive)) as Directive;
}

/** The removal a directive asks for, if it is one. */
export function removalOf(directive: Directive | undefined): Removal | undefined {
  if (
    directive?.type === "delete" &&
    directive.messageId !== undefined &&
    directive.channelId !== undefined
  ) {
    return { kind: "delete", messageId: directive.messageId, channelId: directive.channelId };
  }
  if (directive?.type === "purge" && directive.userId !== undefined && directive.since !== undefined) {
    return { kind: "purge", userId: directive.userId, since: directive.since };
  }
  return undefined;
}

/** Whether a removal takes the channel's message. `userOf` names a sender's user ID. */
export function removes(
  removal: Removal,
  channelId: string,
  message: Pick<DecryptedMessage, "id" | "sender" | "sentAt">,
  userOf: (sender: string) => string | undefined,
): boolean {
  if (removal.kind === "delete") {
    return removal.channelId === channelId && removal.messageId === message.id;
  }
  return Date.parse(message.sentAt) >= Date.parse(removal.since) && userOf(message.sender) === removal.userId;
}

/** Drops what the removals take from a channel's stored messages and the search index. */
export async function applyRemovals(
  context: EngineContext,
  channelId: string,
  removals: readonly Removal[],
  userOf: (sender: string) => string | undefined,
): Promise<void> {
  if (removals.length === 0) {
    return;
  }
  const collection = `messages:${channelId}`;
  const gone: string[] = [];
  for (const key of await context.database.list(collection)) {
    const stored = await context.database.get(collection, key);
    const message =
      stored === undefined ? undefined : (JSON.parse(new TextDecoder().decode(stored)) as DecryptedMessage);
    if (message !== undefined && removals.some((removal) => removes(removal, channelId, message, userOf))) {
      await context.database.delete(collection, key);
      gone.push(message.id);
    }
  }
  await unindexMessages(context.database, gone);
}
