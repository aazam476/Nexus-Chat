/**
 * Server channels (docs/e2ee-design.md §3.1 and §6). A text channel's
 * messages travel in the MLS group of its audience, everyone who can see
 * it, and channels with the same audience share one group. Syncing that
 * group sends each message to its channel, after checking that the channel
 * the server tagged it with is the one inside the encryption.
 *
 * Each channel has its own history keys (the exporter label includes the
 * channel ID), named by group and epoch, since a channel moves between
 * groups as its audience changes. When someone new joins the group, a
 * device that holds a channel's keys sends them a history bundle, for each
 * channel whose permissions let them read history from before they joined;
 * it does not if another member's bundle for that channel came after the
 * join. A bundle received adds its keys to this device's and to the backup.
 */
import {
  decodeHistoryBundle,
  encodeHistoryBundle,
  encrypt,
  epoch,
  groupId,
  historyKey,
  type Group,
} from "@nexus-chat/crypto";

import { decodePayload } from "../payload";
import type { DeliveredMessage } from "./api";
import { keptHistoryKeys, saveHistoryKeys } from "./archive";
import { fromBase64, toBase64, utf8 } from "./bytes";
import type { EngineContext } from "./context";
import { storeMessages } from "./history";
import { memberUsers, syncConversation, type AudienceHooks, type DecryptedMessage } from "./sync";

/** A server audience as this device sees it. */
export interface Audience {
  serverId: string;
  /** The audience's conversation, whose group carries its channels. */
  audienceId: string;
  /** Its text channels. */
  channelIds: readonly string[];
  /** Whether userId may read channelId's messages from before they joined it. */
  mayReadHistory(userId: string, channelId: string): boolean;
}

/** Where a channel's history keys are kept, on the device and in the backup. */
export const channelKeysId = (channelId: string): string => `channel.${channelId}`;

/** A history key's name: the group and the epoch. */
export const keyName = (groupId: Uint8Array, at: bigint | number): string =>
  `${toBase64(groupId)}:${String(at)}`;

/** Every history key this device keeps for the channel, as bundle entries. */
async function bundleEntries(
  context: EngineContext,
  channelId: string,
): Promise<{ groupId: Uint8Array; epoch: bigint; key: Uint8Array }[]> {
  const kept = await keptHistoryKeys(context, channelKeysId(channelId));
  return Object.entries(kept).flatMap(([name, key]) => {
    const [group, at] = name.split(":");
    return group === undefined || at === undefined
      ? []
      : [{ groupId: fromBase64(group), epoch: BigInt(at), key: fromBase64(key) }];
  });
}

/** One sync pass over an audience's group. */
class AudienceSync implements AudienceHooks {
  private readonly keys = new Map<string, Record<string, string>>();
  private readonly owed = new Set<string>();
  private readonly bundled = new Set<string>();

  constructor(
    private readonly context: EngineContext,
    private readonly audience: Audience,
  ) {
    for (const channelId of audience.channelIds) {
      this.keys.set(channelId, {});
    }
  }

  received(plaintext: Uint8Array, message: DeliveredMessage, sender: string, out: DecryptedMessage[]): void {
    const channelId = message.channelId;
    const noted = channelId === undefined ? undefined : this.keys.get(channelId);
    if (channelId === undefined || noted === undefined) {
      return;
    }
    const bundle = decodeHistoryBundle(plaintext);
    if (bundle !== undefined) {
      if (new TextDecoder().decode(bundle.channelId) === channelId) {
        for (const entry of bundle.entries) {
          noted[keyName(entry.groupId, entry.epoch)] = toBase64(entry.key);
        }
        this.owed.delete(channelId);
        this.bundled.add(channelId);
      }
      return;
    }
    const envelope = decodePayload(plaintext);
    if (envelope?.channel === channelId) {
      out.push({
        id: envelope.id,
        sender,
        payload: envelope.payload,
        sentAt: message.createdAt,
        deliveryId: message.id,
        channelId,
      });
    }
  }

  committed(before: Group, after: Group): void {
    const had = memberUsers(this.context, before);
    for (const user of memberUsers(this.context, after)) {
      if (had.has(user)) {
        continue;
      }
      for (const channelId of this.audience.channelIds) {
        if (this.audience.mayReadHistory(user, channelId)) {
          this.owed.add(channelId);
        }
      }
    }
  }

  async noteKeys(group: Group): Promise<void> {
    for (const [channelId, noted] of this.keys) {
      noted[keyName(groupId(group), epoch(group))] ??= toBase64(await historyKey(group, utf8(channelId)));
    }
  }

  async finish(group: Group | undefined, out: DecryptedMessage[]): Promise<Group | undefined> {
    for (const [channelId, noted] of this.keys) {
      await storeMessages(
        this.context,
        channelId,
        out.filter((message) => message.channelId === channelId),
      );
      await saveHistoryKeys(this.context, channelKeysId(channelId), noted);
    }
    let current = group;
    for (const channelId of this.owed) {
      current = current === undefined ? undefined : await this.sendBundle(current, channelId);
    }
    return current;
  }

  /** The channels a bundle brought keys for in this pass. */
  get channelsWithNewKeys(): ReadonlySet<string> {
    return this.bundled;
  }

  /** Sends the channel's keys to the group, for its newest members. */
  private async sendBundle(group: Group, channelId: string): Promise<Group> {
    const entries = await bundleEntries(this.context, channelId);
    if (entries.length === 0) {
      return group;
    }
    const encrypted = await encrypt(group, encodeHistoryBundle(utf8(channelId), entries));
    const outcome = await this.context.api.sendChannelMessage(
      { serverId: this.audience.serverId, channelId },
      encrypted.message,
    );
    // If the group moved on, another member sends it, or this device on its next sync.
    return outcome.accepted ? encrypted.group : group;
  }
}

/** The hooks for one sync pass over the audience's group. */
export function audienceHooks(context: EngineContext, audience: Audience): AudienceSync {
  return new AudienceSync(context, audience);
}

/**
 * Applies everything new in the audience's group: each channel's messages
 * are stored with the channel, and it returns the channels that a history
 * bundle brought keys for, whose history can now be read.
 */
export async function syncAudience(
  context: EngineContext,
  audience: Audience,
): Promise<{ messages: DecryptedMessage[]; newHistory: ReadonlySet<string> }> {
  const hooks = audienceHooks(context, audience);
  const messages = await syncConversation(context, audience.audienceId, hooks);
  return { messages, newHistory: hooks.channelsWithNewKeys };
}
