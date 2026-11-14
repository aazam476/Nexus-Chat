/**
 * Joining a server audience's group, sending in its channels, and reading
 * a channel's history from the archive (docs/e2ee-design.md §3–4, §6).
 *
 * The first device to open an audience creates its group alone; every
 * other device joins by external commit, which the server allows for
 * everyone in the audience. A channel message carries its channel inside
 * the encryption, and its archive copy is sealed under the channel's
 * history key.
 */
import { encrypt, epoch, groupId, historyKey, sealArchive, type Group } from "@nexus-chat/crypto";

import { encodePayload, type Payload } from "../payload";
import type { Outcome } from "./api";
import { keptHistoryKeys, openCopy, saveHistoryKeys } from "./archive";
import { toBase64, utf8 } from "./bytes";
import { audienceHooks, channelKeysId, keyName, syncAudience, type Audience } from "./channels";
import { loadState, saveState, type EngineContext } from "./context";
import { loadMessages, storeMessages } from "./history";
import { ConversationError, startConversation } from "./send";
import type { DecryptedMessage } from "./sync";

/** How many pages of a channel's archive are read at most. */
const maximumPages = 50;

/** Makes this device a member of the audience's group. */
export async function joinAudience(context: EngineContext, audience: Audience): Promise<void> {
  await startConversation(context, audience.audienceId, [], audienceHooks(context, audience));
}

/** A channel message on its way out. */
interface ChannelMessage {
  id: string;
  channelId: string;
  payload: Payload;
}

/** Keeps a sent message and the history key it was sealed under, from the group it was sent in. */
async function keepSent(
  context: EngineContext,
  outgoing: ChannelMessage,
  sentIn: Group,
  deliveryId?: number,
): Promise<void> {
  const { id, channelId, payload } = outgoing;
  await saveHistoryKeys(context, channelKeysId(channelId), {
    [keyName(groupId(sentIn), epoch(sentIn))]: toBase64(await historyKey(sentIn, utf8(channelId))),
  });
  const own: DecryptedMessage = {
    id,
    sender: context.device.address,
    payload,
    sentAt: new Date().toISOString(),
    channelId,
  };
  await storeMessages(context, channelId, [deliveryId === undefined ? own : { ...own, deliveryId }]);
}

/** One attempt at sending; on success the device keeps the message and its key. */
async function trySend(
  context: EngineContext,
  audience: Audience,
  outgoing: ChannelMessage,
): Promise<Outcome> {
  const { id, channelId, payload } = outgoing;
  const state = await loadState(context, audience.audienceId);
  if (state.group === undefined) {
    throw new ConversationError("this device is not in the channel's group yet");
  }
  const plaintext = encodePayload({ id, payload, channel: channelId });
  const archive = await sealArchive(
    state.group,
    { channelId: utf8(channelId), messageId: utf8(id) },
    plaintext,
  );
  const encrypted = await encrypt(state.group, plaintext);
  const outcome = await context.api.sendChannelMessage(
    { serverId: audience.serverId, channelId },
    encrypted.message,
    {
      messageId: id,
      sealed: archive.sealed,
    },
  );
  if (!outcome.accepted) {
    return outcome;
  }
  await saveState(context, audience.audienceId, { ...state, group: encrypted.group });
  await keepSent(context, outgoing, state.group, outcome.id);
  return outcome;
}

/**
 * Sends a payload in one of the audience's channels and returns its
 * message ID. If the group moved on meanwhile, it catches up and tries
 * once more.
 */
export async function sendToChannel(
  context: EngineContext,
  audience: Audience,
  channelId: string,
  payload: Payload,
): Promise<string> {
  const outgoing = { id: crypto.randomUUID(), channelId, payload };
  for (let attempt = 0; attempt < 2; attempt++) {
    const outcome = await trySend(context, audience, outgoing);
    if (outcome.accepted) {
      return outgoing.id;
    }
    if (outcome.reason === "refused") {
      break;
    }
    await syncAudience(context, audience);
  }
  throw new ConversationError("the message could not be sent; try again");
}

/**
 * Reads the channel's archive with the history keys this device holds,
 * its own and those that came in bundles, and keeps what it had not.
 * It returns how many messages it added.
 */
export async function readChannelHistory(
  context: EngineContext,
  audience: Audience,
  channelId: string,
): Promise<number> {
  const keys = await keptHistoryKeys(context, channelKeysId(channelId));
  const held = new Set((await loadMessages(context, channelId)).map((message) => message.id));
  const found: DecryptedMessage[] = [];
  let before: string | undefined;
  for (let page = 0; page < maximumPages; page++) {
    const copies = await context.api.channelArchive({ serverId: audience.serverId, channelId }, before);
    for (const copy of copies) {
      const message = held.has(copy.messageId)
        ? undefined
        : await openCopy(context, channelId, copy, keys[keyName(copy.groupId, copy.epoch)]);
      if (message !== undefined) {
        found.push({ ...message, channelId });
        held.add(message.id);
      }
    }
    before = copies.at(-1)?.createdAt;
    if (before === undefined) {
      break;
    }
  }
  await storeMessages(context, channelId, found);
  return found.length;
}
