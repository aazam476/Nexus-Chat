/**
 * An in-memory instance for messaging-engine tests: identities, key
 * backup, key packages and the delivery service, with the Go server's
 * rules (one commit per epoch; messages only for the current epoch). Each
 * `api(user, device)` is one signed-in device.
 */
import { messageEpoch, type PublishedIdentity } from "@nexus-chat/crypto";

import type {
  ChannelAddress,
  DeliveredMessage,
  MessagingApi,
  Outcome,
  StoredArchive,
  StoredChannelArchive,
} from "../messaging/engine/api";

interface Group {
  groupId: Uint8Array;
  epoch: number;
  groupInfo?: Uint8Array;
}

/** The epoch an MLS group message was made in. */
function epochOf(bytes: Uint8Array): number | undefined {
  const epoch = messageEpoch(bytes);
  return epoch === undefined ? undefined : Number(epoch);
}

/** A fake instance shared by every test device. */
export class FakeInstance {
  identities = new Map<string, PublishedIdentity>();
  backups = new Map<
    string,
    { salt: Uint8Array; wrapped: Map<string, Uint8Array>; blobs: Map<string, Uint8Array> }
  >();
  keyPackages = new Map<string, { deviceId: string; keyPackage: Uint8Array; lastResort: boolean }[]>();
  groups = new Map<string, Group>();
  messages = new Map<string, DeliveredMessage[]>();
  /** Archive copies per conversation, oldest first. */
  archives = new Map<string, StoredArchive[]>();
  /** Each server channel's audience conversation; `moveChannel` changes it. */
  channels = new Map<string, string>();
  /** Archive copies per channel, oldest first. */
  channelArchives = new Map<string, StoredChannelArchive[]>();
  /** Channel messages sent without an archive copy: history bundles. */
  bundles = 0;
  private nextId = 1;

  /** Adds a text channel whose messages go to the audience's group. */
  addChannel(channelId: string, audienceId: string): void {
    if (!this.groups.has(audienceId)) {
      this.addConversation(audienceId);
    }
    this.channels.set(channelId, audienceId);
  }

  /** Adds a conversation, as the server does when a DM is opened. */
  addConversation(conversationId: string): void {
    this.groups.set(conversationId, { groupId: crypto.getRandomValues(new Uint8Array(16)), epoch: 0 });
    this.messages.set(conversationId, []);
  }

  /** Stores a message as the delivery service would. */
  deliver(conversationId: string, message: Omit<DeliveredMessage, "id" | "createdAt">): number {
    const id = this.nextId++;
    this.messages.get(conversationId)?.push({ ...message, id, createdAt: new Date(id * 1000).toISOString() });
    return id;
  }

  private backupOf(userId: string): {
    salt: Uint8Array;
    wrapped: Map<string, Uint8Array>;
    blobs: Map<string, Uint8Array>;
  } {
    let backup = this.backups.get(userId);
    if (backup === undefined) {
      backup = { salt: crypto.getRandomValues(new Uint8Array(16)), wrapped: new Map(), blobs: new Map() };
      this.backups.set(userId, backup);
    }
    return backup;
  }

  private commit(
    conversationId: string,
    userId: string,
    commit: { commit: Uint8Array; groupInfo: Uint8Array; welcome?: Uint8Array },
  ): Outcome {
    const group = this.groups.get(conversationId);
    if (group === undefined || epochOf(commit.commit) !== group.epoch) {
      return { accepted: false, reason: "stale" };
    }
    this.deliver(conversationId, {
      kind: "commit",
      epoch: group.epoch,
      senderId: userId,
      data: commit.commit,
    });
    if (commit.welcome !== undefined) {
      this.deliver(conversationId, {
        kind: "welcome",
        epoch: group.epoch + 1,
        senderId: userId,
        data: commit.welcome,
      });
    }
    group.epoch += 1;
    group.groupInfo = commit.groupInfo;
    return { accepted: true };
  }

  private send(
    conversationId: string,
    userId: string,
    sent: { message: Uint8Array; archive: { messageId: string; sealed: Uint8Array } },
  ): Outcome {
    const group = this.groups.get(conversationId);
    if (group === undefined || epochOf(sent.message) !== group.epoch) {
      return { accepted: false, reason: "stale" };
    }
    const id = this.deliver(conversationId, {
      kind: "application",
      epoch: group.epoch,
      senderId: userId,
      data: sent.message,
    });
    const copies = this.archives.get(conversationId) ?? [];
    copies.push({
      ...sent.archive,
      epoch: group.epoch,
      senderId: userId,
      createdAt: new Date(id * 1000).toISOString(),
    });
    this.archives.set(conversationId, copies);
    return { accepted: true, id };
  }

  private sendToChannel(
    channel: ChannelAddress,
    userId: string,
    sent: { message: Uint8Array; archive?: { messageId: string; sealed: Uint8Array } },
  ): Outcome {
    const audienceId = this.channels.get(channel.channelId);
    const group = audienceId === undefined ? undefined : this.groups.get(audienceId);
    if (audienceId === undefined || group === undefined) {
      return { accepted: false, reason: "refused" };
    }
    if (epochOf(sent.message) !== group.epoch) {
      return { accepted: false, reason: "stale" };
    }
    const id = this.deliver(audienceId, {
      kind: "application",
      epoch: group.epoch,
      senderId: userId,
      data: sent.message,
      channelId: channel.channelId,
    });
    if (sent.archive === undefined) {
      this.bundles += 1;
    } else {
      const copies = this.channelArchives.get(channel.channelId) ?? [];
      copies.push({
        ...sent.archive,
        groupId: group.groupId,
        epoch: group.epoch,
        senderId: userId,
        createdAt: new Date(id * 1000).toISOString(),
      });
      this.channelArchives.set(channel.channelId, copies);
    }
    return { accepted: true, id };
  }

  /** Archive copies before `before`, newest first, two at a time to exercise paging. */
  private archiveBefore(conversationId: string, before?: string): StoredArchive[] {
    return (this.archives.get(conversationId) ?? [])
      .filter((copy) => before === undefined || copy.createdAt < before)
      .reverse()
      .slice(0, 2);
  }

  private claim(userId: string): { deviceId: string; keyPackage: Uint8Array }[] {
    const stock = this.keyPackages.get(userId) ?? [];
    const byDevice = new Map<string, { deviceId: string; keyPackage: Uint8Array }>();
    for (const entry of [...stock].sort((a, b) => Number(a.lastResort) - Number(b.lastResort))) {
      if (!byDevice.has(entry.deviceId)) {
        byDevice.set(entry.deviceId, entry);
        if (!entry.lastResort) {
          stock.splice(stock.indexOf(entry), 1);
        }
      }
    }
    return [...byDevice.values()].map(({ deviceId, keyPackage }) => ({ deviceId, keyPackage }));
  }

  private groupOf(conversationId: string): Group {
    const group = this.groups.get(conversationId);
    if (group === undefined) {
      throw new Error(`no conversation ${conversationId}`);
    }
    return group;
  }

  private identityApi(
    userId: string,
  ): Pick<
    MessagingApi,
    "getIdentity" | "publishIdentity" | "getBackup" | "putWrappedKey" | "putBlob" | "getBlob"
  > {
    return {
      getIdentity: (id) => Promise.resolve(this.identities.get(id)),
      publishIdentity: (identity) => {
        this.identities.set(userId, identity);
        return Promise.resolve();
      },
      getBackup: () => {
        const backup = this.backupOf(userId);
        return Promise.resolve({
          salt: backup.salt,
          wrapped: backup.wrapped,
          blobs: [...backup.blobs.keys()],
        });
      },
      putWrappedKey: (method, wrapped) => {
        this.backupOf(userId).wrapped.set(method, wrapped);
        return Promise.resolve();
      },
      putBlob: (name, data) => {
        this.backupOf(userId).blobs.set(name, data);
        return Promise.resolve();
      },
      getBlob: (name) => Promise.resolve(this.backupOf(userId).blobs.get(name)),
    };
  }

  private keyPackageApi(
    userId: string,
    deviceId: string,
  ): Pick<MessagingApi, "keyPackageStock" | "uploadKeyPackages" | "claimKeyPackages"> {
    return {
      keyPackageStock: () => {
        const mine = (this.keyPackages.get(userId) ?? []).filter((entry) => entry.deviceId === deviceId);
        const oneTime = mine.filter((entry) => !entry.lastResort).length;
        return Promise.resolve({
          oneTime,
          refill: oneTime < 20,
          hasLastResort: mine.some((entry) => entry.lastResort),
        });
      },
      uploadKeyPackages: (oneTime, lastResort) => {
        const stock = this.keyPackages.get(userId) ?? [];
        stock.push(...oneTime.map((keyPackage) => ({ deviceId, keyPackage, lastResort: false })));
        if (lastResort !== undefined) {
          stock.push({ deviceId, keyPackage: lastResort, lastResort: true });
        }
        this.keyPackages.set(userId, stock);
        return Promise.resolve();
      },
      claimKeyPackages: (id) => Promise.resolve(this.claim(id)),
    };
  }

  /** The API one device of `userId` sees. */
  api(userId: string, deviceId: string): MessagingApi {
    return {
      ...this.identityApi(userId),
      ...this.keyPackageApi(userId, deviceId),
      groupState: (conversationId) => Promise.resolve({ ...this.groupOf(conversationId) }),
      submitCommit: (conversationId, commit) => Promise.resolve(this.commit(conversationId, userId, commit)),
      sendMessage: (conversationId, message, archive) =>
        Promise.resolve(this.send(conversationId, userId, { message, archive })),
      messages: (conversationId, afterId) =>
        Promise.resolve((this.messages.get(conversationId) ?? []).filter((message) => message.id > afterId)),
      archive: (conversationId, before) => Promise.resolve(this.archiveBefore(conversationId, before)),
      sendChannelMessage: (channel, message, archive) =>
        Promise.resolve(
          this.sendToChannel(channel, userId, { message, ...(archive === undefined ? {} : { archive }) }),
        ),
      channelArchive: (channel, before) =>
        Promise.resolve(
          (this.channelArchives.get(channel.channelId) ?? [])
            .filter((copy) => before === undefined || copy.createdAt < before)
            .reverse()
            .slice(0, 2),
        ),
    };
  }
}
