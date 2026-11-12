/**
 * Bringing a conversation up to date: fetching what the delivery service
 * holds after this device's cursor and applying it in order
 * (docs/e2ee-design.md §3–4). Commits move the group to the next epoch,
 * a Welcome makes the device a member, application messages are decrypted
 * for the screen, and membership directives make the device commit the
 * removals the instance signed. Anything that fails to apply is skipped:
 * members reject what is not valid MLS.
 */
import {
  epoch,
  joinFromWelcome,
  loadGroup,
  members,
  receive,
  removeMembers,
  saveGroup,
  userOf,
  decodeDeviceCredential,
  verifyDirective,
  type Group,
} from "@nexus-chat/crypto";

import { decodePayload, type Payload } from "../payload";
import type { DeliveredMessage } from "./api";
import { fromBase64 } from "./bytes";
import { loadState, saveState, type EngineContext } from "./context";
import { forgetMember, heldMembers } from "./device";
import { storeMessages } from "./history";

/** A decrypted message, ready for the conversation view. */
export interface DecryptedMessage {
  id: string;
  /** The sender device's address, as MLS authenticated it. */
  sender: string;
  payload: Payload;
  sentAt: string;
  /** The live copy's ID on the delivery service, which read state counts by; missing in messages saved before it was kept. */
  deliveryId?: number;
}

async function tryWelcome(context: EngineContext, welcome: Uint8Array): Promise<Group | undefined> {
  for (const { id, member } of await heldMembers(context.database)) {
    try {
      const group = await joinFromWelcome(welcome, member, context.check);
      await forgetMember(context.database, id);
      return group;
    } catch {
      // Not for this key package; try the next.
    }
  }
  return undefined;
}

async function decrypt(group: Group, message: DeliveredMessage, out: DecryptedMessage[]): Promise<Group> {
  const received = await receive(group, message.data).catch(() => undefined);
  if (received?.kind !== "message") {
    return received?.group ?? group;
  }
  const envelope = decodePayload(received.plaintext);
  const sender =
    received.sender === undefined ? undefined : decodeDeviceCredential(received.sender)?.identity;
  if (envelope !== undefined && sender !== undefined) {
    out.push({
      id: envelope.id,
      sender,
      payload: envelope.payload,
      sentAt: message.createdAt,
      deliveryId: message.id,
    });
  }
  return received.group;
}

/** The user a validly signed removal directive removes, or undefined. */
async function directiveTarget(context: EngineContext, data: Uint8Array): Promise<string | undefined> {
  const signed = JSON.parse(new TextDecoder().decode(data)) as { directive?: string; signature?: string };
  const directive = fromBase64(signed.directive ?? "");
  if (!(await verifyDirective(context.instanceKey, directive, fromBase64(signed.signature ?? "")))) {
    return undefined;
  }
  const parsed = JSON.parse(new TextDecoder().decode(directive)) as { type?: string; userId?: string };
  return parsed.type === "remove" ? parsed.userId : undefined;
}

/** The user ID behind a device address, through the conversation's members. */
function userIdOf(context: EngineContext, address: string | undefined): string | undefined {
  const user = address === undefined ? undefined : userOf(address);
  return user === undefined ? undefined : context.userOfAddress(user);
}

/** The leaves of the group that belong to userId's devices. */
function leavesOf(context: EngineContext, group: Group, userId: string): number[] {
  const leaves: number[] = [];
  for (const [leaf, identity] of members(group)) {
    if (userIdOf(context, decodeDeviceCredential(identity)?.identity) === userId) {
      leaves.push(leaf);
    }
  }
  return leaves;
}

/**
 * Acts on a membership directive: a device of the removed user forgets
 * the group; any other member commits the removal.
 */
async function applyDirective(
  context: EngineContext,
  conversationId: string,
  group: Group,
  data: Uint8Array,
): Promise<Group | undefined> {
  const target = await directiveTarget(context, data).catch(() => undefined);
  if (target === undefined) {
    return group;
  }
  if (target === userIdOf(context, context.device.address)) {
    return undefined;
  }
  const leaves = leavesOf(context, group, target);
  if (leaves.length === 0) {
    return group;
  }
  // Commit on a copy: making a commit wipes key material the original
  // still needs if another member's removal is accepted first.
  const removal = await removeMembers(loadGroup(saveGroup(group), context.check), leaves);
  const outcome = await context.api.submitCommit(conversationId, removal);
  // If another member's removal came first, its commit arrives next.
  return outcome.accepted ? removal.group : group;
}

/** One sync pass over a conversation: where it applies, and what it decrypted so far. */
interface Pass {
  context: EngineContext;
  conversationId: string;
  out: DecryptedMessage[];
}

async function apply(
  pass: Pass,
  group: Group | undefined,
  message: DeliveredMessage,
): Promise<Group | undefined> {
  const { context, conversationId, out } = pass;
  if (group === undefined) {
    return message.kind === "welcome" ? tryWelcome(context, message.data) : undefined;
  }
  switch (message.kind) {
    case "commit": {
      // Our own commits are already applied: they are for an earlier epoch.
      if (message.epoch !== Number(epoch(group))) {
        return group;
      }
      const received = await receive(group, message.data).catch(() => undefined);
      return received?.group ?? group;
    }
    case "application":
      return decrypt(group, message, out);
    case "directive":
      return applyDirective(context, conversationId, group, message.data);
    default:
      return group;
  }
}

/** Applies everything new in the conversation and returns the messages decrypted. */
export async function syncConversation(
  context: EngineContext,
  conversationId: string,
): Promise<DecryptedMessage[]> {
  const state = await loadState(context, conversationId);
  const out: DecryptedMessage[] = [];
  for (const message of await context.api.messages(conversationId, state.cursor)) {
    state.group = await apply({ context, conversationId, out }, state.group, message);
    state.cursor = message.id;
  }
  await storeMessages(context, conversationId, out);
  await saveState(context, conversationId, state);
  return out;
}
