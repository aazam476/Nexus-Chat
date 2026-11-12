/**
 * Starting, joining and sending in a conversation (docs/e2ee-design.md
 * §3.2 and §4).
 *
 * The first device to open a new conversation creates its MLS group and
 * adds one key package of every other member device. Any device that
 * finds the group already running joins it by external commit from the
 * GroupInfo the server holds. A message goes out as its live MLS copy and
 * its archive copy; if the group moved on meanwhile, the device catches up
 * and sends again.
 */
import {
  addMembers,
  createGroup,
  createMember,
  encodeGroupInfo,
  encrypt,
  historyKey,
  joinExternal,
  sealArchive,
  type Group,
} from "@nexus-chat/crypto";

import { encodePayload, type Payload } from "../payload";
import { noteHistoryKey, saveHistoryKeys } from "./archive";
import { utf8 } from "./bytes";
import { loadState, saveState, type EngineContext } from "./context";
import { storeMessages } from "./history";
import { syncConversation } from "./sync";

/** Thrown when the conversation cannot be joined or sent to now. */
export class ConversationError extends Error {
  override name = "ConversationError";
}

/** One key package of every member device except this one. */
async function claimAll(context: EngineContext, memberIds: string[]): Promise<Uint8Array[]> {
  const packages: Uint8Array[] = [];
  for (const userId of memberIds) {
    for (const claimed of await context.api.claimKeyPackages(userId)) {
      if (claimed.deviceId !== context.device.deviceId) {
        packages.push(claimed.keyPackage);
      }
    }
  }
  return packages;
}

async function create(
  context: EngineContext,
  conversationId: string,
  groupId: Uint8Array,
  memberIds: string[],
): Promise<Group | undefined> {
  const member = await createMember(context.device.credential, context.device.signatureKeys);
  const group = await createGroup(groupId, member, context.check);
  const added = await addMembers(group, await claimAll(context, memberIds));
  const outcome = await context.api.submitCommit(conversationId, added);
  return outcome.accepted ? added.group : undefined;
}

async function join(
  context: EngineContext,
  conversationId: string,
  groupInfo: Uint8Array,
): Promise<Group | undefined> {
  const member = await createMember(context.device.credential, context.device.signatureKeys);
  const joined = await joinExternal(groupInfo, member, context.check);
  // The server needs the new epoch's GroupInfo, which now includes this device.
  const outcome = await context.api.submitCommit(conversationId, {
    commit: joined.commit,
    groupInfo: await encodeGroupInfo(joined.group.state),
  });
  return outcome.accepted ? joined.group : undefined;
}

/** Backs up the history key of the group's current epoch. */
async function backUpCurrentKey(context: EngineContext, conversationId: string, group: Group): Promise<void> {
  const keys: Record<string, string> = {};
  await noteHistoryKey(conversationId, group, keys);
  await saveHistoryKeys(context, conversationId, keys);
}

/**
 * Makes this device a member of the conversation's group: catching up on
 * a Welcome, creating the group, or joining it. Tries twice, since another
 * device may change the group at the same moment.
 */
export async function startConversation(
  context: EngineContext,
  conversationId: string,
  memberIds: string[],
): Promise<void> {
  for (let attempt = 0; attempt < 2; attempt++) {
    // The group state first: a Welcome for this device that came with it is
    // then always applied by the sync, rather than joining a second time.
    const server = await context.api.groupState(conversationId);
    await syncConversation(context, conversationId);
    const state = await loadState(context, conversationId);
    if (state.group !== undefined) {
      return;
    }
    const group =
      server.groupInfo === undefined
        ? await create(context, conversationId, server.groupId, memberIds)
        : await join(context, conversationId, server.groupInfo);
    if (group !== undefined) {
      await saveState(context, conversationId, { ...state, group });
      await backUpCurrentKey(context, conversationId, group);
      return;
    }
  }
  throw new ConversationError("could not join the conversation; try again");
}

/** Sends a payload and returns its message ID. */
export async function sendPayload(
  context: EngineContext,
  conversationId: string,
  payload: Payload,
): Promise<string> {
  const id = crypto.randomUUID();
  const plaintext = encodePayload({ id, payload });
  const address = { channelId: utf8(conversationId), messageId: utf8(id) };
  for (let attempt = 0; attempt < 2; attempt++) {
    const state = await loadState(context, conversationId);
    if (state.group === undefined) {
      throw new ConversationError("this device is not in the conversation yet");
    }
    const archive = await sealArchive(state.group, address, plaintext);
    const encrypted = await encrypt(state.group, plaintext);
    const outcome = await context.api.sendMessage(conversationId, encrypted.message, {
      messageId: id,
      sealed: archive.sealed,
    });
    if (outcome.accepted) {
      await saveState(context, conversationId, { ...state, group: encrypted.group });
      const sentAt = new Date().toISOString();
      const own = { id, sender: context.device.address, payload, sentAt };
      await storeMessages(context, conversationId, [
        outcome.id === undefined ? own : { ...own, deliveryId: outcome.id },
      ]);
      return id;
    }
    await syncConversation(context, conversationId);
  }
  throw new ConversationError("the message could not be sent; try again");
}

/** The history key of the conversation's current epoch, for the key backup. */
export async function currentHistoryKey(
  context: EngineContext,
  conversationId: string,
): Promise<Uint8Array | undefined> {
  const state = await loadState(context, conversationId);
  return state.group === undefined ? undefined : historyKey(state.group, utf8(conversationId));
}
