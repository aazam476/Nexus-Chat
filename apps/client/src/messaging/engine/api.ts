/**
 * What the messaging engine needs from the server, as one small interface
 * in the engine's own terms (bytes, not base64), so the engine can be
 * tested against an in-memory delivery service. `createMessagingApi`
 * implements it with the generated client.
 */
import type { PublishedIdentity } from "@nexus-chat/crypto";

/** A live MLS message as the delivery service hands it out. */
export interface DeliveredMessage {
  id: number;
  kind: "commit" | "application" | "welcome" | "directive";
  epoch: number;
  senderId?: string;
  data: Uint8Array;
  createdAt: string;
  /** In a server audience's group, the channel an application message is for. */
  channelId?: string;
}

/** A server text channel, as channel calls address it. */
export interface ChannelAddress {
  serverId: string;
  channelId: string;
}

/** A channel's archive copy, with the MLS group it was sent in. */
export interface StoredChannelArchive {
  messageId: string;
  groupId: Uint8Array;
  epoch: number;
  senderId?: string;
  sealed: Uint8Array;
  createdAt: string;
}

/** An archive copy as stored. */
export interface StoredArchive {
  messageId: string;
  epoch: number;
  senderId?: string;
  sealed: Uint8Array;
  createdAt: string;
}

/** The group as the delivery service tracks it. */
export interface DeliveryGroup {
  groupId: Uint8Array;
  epoch: number;
  groupInfo?: Uint8Array;
}

/** How a commit or send ended: accepted, or stale (fetch and retry). */
export type Outcome = { accepted: true; id?: number } | { accepted: false; reason: "stale" | "refused" };

/** The server calls the engine makes. */
export interface MessagingApi {
  getIdentity(userId: string): Promise<PublishedIdentity | undefined>;
  publishIdentity(identity: PublishedIdentity): Promise<void>;
  getBackup(): Promise<{ salt: Uint8Array; wrapped: Map<string, Uint8Array>; blobs: string[] }>;
  putWrappedKey(method: "recovery-phrase" | "password" | "passkey", wrapped: Uint8Array): Promise<void>;
  putBlob(name: string, data: Uint8Array): Promise<void>;
  getBlob(name: string): Promise<Uint8Array | undefined>;
  keyPackageStock(): Promise<{ oneTime: number; refill: boolean; hasLastResort: boolean }>;
  uploadKeyPackages(oneTime: Uint8Array[], lastResort?: Uint8Array): Promise<void>;
  /** One key package per device of the user, with the device's ID. */
  claimKeyPackages(userId: string): Promise<{ deviceId: string; keyPackage: Uint8Array }[]>;
  groupState(conversationId: string): Promise<DeliveryGroup>;
  submitCommit(
    conversationId: string,
    commit: { commit: Uint8Array; groupInfo: Uint8Array; welcome?: Uint8Array },
  ): Promise<Outcome>;
  sendMessage(
    conversationId: string,
    message: Uint8Array,
    archive: { messageId: string; sealed: Uint8Array },
  ): Promise<Outcome>;
  messages(conversationId: string, afterId: number): Promise<DeliveredMessage[]>;
  archive(conversationId: string, before?: string): Promise<StoredArchive[]>;
  /** Sends in a server channel, through its audience's group; history bundles have no archive copy. */
  sendChannelMessage(
    channel: ChannelAddress,
    message: Uint8Array,
    archive?: { messageId: string; sealed: Uint8Array },
  ): Promise<Outcome>;
  channelArchive(channel: ChannelAddress, before?: string): Promise<StoredChannelArchive[]>;
}
