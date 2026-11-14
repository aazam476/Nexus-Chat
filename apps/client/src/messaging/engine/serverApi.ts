/**
 * MessagingApi over the generated HTTP client. Bytes become base64 on the
 * way out and back on the way in; a 409 from the delivery service (another
 * commit won, or the message is for an old epoch) becomes a stale outcome
 * the engine handles by fetching and retrying. Any other failure throws.
 */
import {
  claimKeyPackages,
  getBackupBlob,
  getIdentity,
  getKeyBackup,
  getKeyPackageStock,
  getMlsGroupState,
  listArchivedMessages,
  listChannelArchive,
  sendChannelMessage,
  listMlsMessages,
  publishIdentity,
  putBackupBlob,
  putWrappedBackupKey,
  sendMlsMessage,
  submitMlsCommit,
  uploadKeyPackages,
  type Client,
} from "@nexus-chat/api-client";

import type { MessagingApi, Outcome } from "./api";
import { fromBase64, toBase64 } from "./bytes";

/** Thrown when the server answers something the engine cannot go on from. */
export class ServerError extends Error {
  override name = "ServerError";
}

function required<T>(data: T | undefined, what: string): T {
  if (data === undefined) {
    throw new ServerError(`the server refused to ${what}`);
  }
  return data;
}

function outcome(status: number, id?: number): Outcome {
  if (status < 300) {
    return id === undefined ? { accepted: true } : { accepted: true, id };
  }
  if (status === 409) {
    return { accepted: false, reason: "stale" };
  }
  return { accepted: false, reason: "refused" };
}

/** Throws unless the response succeeded. */
function ensureOk(response: Response | undefined, what: string): void {
  if (response?.ok !== true) {
    throw new ServerError(`the server refused to ${what}`);
  }
}

type Calls<Names extends keyof MessagingApi> = Pick<MessagingApi, Names>;

function identityCalls(client: Client): Calls<"getIdentity" | "publishIdentity"> {
  return {
    getIdentity: async (userId) => {
      const result = await getIdentity({ client, path: { userId } });
      if (result.data === undefined) {
        return undefined;
      }
      return {
        masterPublicKey: fromBase64(result.data.masterPublicKey),
        selfSigningPublicKey: fromBase64(result.data.selfSigningPublicKey),
        selfSigningSignature: fromBase64(result.data.selfSigningSignature),
      };
    },
    publishIdentity: async (identity) => {
      const body = {
        masterPublicKey: toBase64(identity.masterPublicKey),
        selfSigningPublicKey: toBase64(identity.selfSigningPublicKey),
        selfSigningSignature: toBase64(identity.selfSigningSignature),
      };
      ensureOk((await publishIdentity({ client, body })).response, "publish the identity");
    },
  };
}

function backupCalls(client: Client): Calls<"getBackup" | "putWrappedKey" | "putBlob" | "getBlob"> {
  return {
    getBackup: async () => {
      const data = required((await getKeyBackup({ client })).data, "read the key backup");
      const wrapped = new Map(
        data.wrappedKeys.map((key) => [key.method as string, fromBase64(key.wrappedKey)]),
      );
      return { salt: fromBase64(data.salt), wrapped, blobs: data.blobs.map((blob) => blob.name) };
    },
    putWrappedKey: async (method, wrapped) => {
      const result = await putWrappedBackupKey({
        client,
        path: { method },
        body: { wrappedKey: toBase64(wrapped) },
      });
      ensureOk(result.response, "store the wrapped backup key");
    },
    putBlob: async (name, data) => {
      ensureOk(
        (await putBackupBlob({ client, path: { name }, body: { data: toBase64(data) } })).response,
        "store a backup blob",
      );
    },
    getBlob: async (name) => {
      const result = await getBackupBlob({ client, path: { name } });
      return result.data === undefined ? undefined : fromBase64(result.data.data);
    },
  };
}

function keyPackageCalls(
  client: Client,
): Calls<"keyPackageStock" | "uploadKeyPackages" | "claimKeyPackages"> {
  return {
    keyPackageStock: async () => required((await getKeyPackageStock({ client })).data, "report key packages"),
    uploadKeyPackages: async (oneTime, lastResort) => {
      const body = {
        keyPackages: oneTime.map(toBase64),
        ...(lastResort === undefined ? {} : { lastResort: toBase64(lastResort) }),
      };
      required((await uploadKeyPackages({ client, body })).data, "store key packages");
    },
    claimKeyPackages: async (userId) => {
      const data = required(
        (await claimKeyPackages({ client, path: { userId } })).data,
        "hand out key packages",
      );
      return data.map((claimed) => ({
        deviceId: claimed.deviceId,
        keyPackage: fromBase64(claimed.keyPackage),
      }));
    },
  };
}

function groupCalls(client: Client): Calls<"groupState" | "submitCommit" | "sendMessage"> {
  return {
    groupState: async (conversationId) => {
      const data = required(
        (await getMlsGroupState({ client, path: { conversationId } })).data,
        "report the group",
      );
      const info = data.groupInfo === undefined ? {} : { groupInfo: fromBase64(data.groupInfo) };
      return { groupId: fromBase64(data.groupId), epoch: data.epoch, ...info };
    },
    submitCommit: async (conversationId, commit) => {
      const welcome = commit.welcome === undefined ? {} : { welcome: toBase64(commit.welcome) };
      const body = { commit: toBase64(commit.commit), groupInfo: toBase64(commit.groupInfo), ...welcome };
      const result = await submitMlsCommit({ client, path: { conversationId }, body });
      return outcome(result.response?.status ?? 0);
    },
    sendMessage: async (conversationId, message, archive) => {
      const body = {
        message: toBase64(message),
        archive: { messageId: archive.messageId, sealed: toBase64(archive.sealed) },
      };
      const result = await sendMlsMessage({ client, path: { conversationId }, body });
      return outcome(result.response?.status ?? 0, result.data?.id);
    },
  };
}

function fetchCalls(client: Client): Calls<"messages" | "archive"> {
  return {
    messages: async (conversationId, afterId) => {
      const result = await listMlsMessages({ client, path: { conversationId }, query: { after: afterId } });
      return required(result.data, "hand out messages").map((message) => ({
        ...message,
        data: fromBase64(message.data),
      }));
    },
    archive: async (conversationId, before) => {
      const result = await listArchivedMessages({
        client,
        path: { conversationId },
        query: before === undefined ? {} : { before },
      });
      return required(result.data, "hand out the archive").map((copy) => ({
        ...copy,
        sealed: fromBase64(copy.sealed),
      }));
    },
  };
}

function channelCalls(client: Client): Calls<"sendChannelMessage" | "channelArchive"> {
  return {
    sendChannelMessage: async (channel, message, archive) => {
      const copy =
        archive === undefined
          ? {}
          : { archive: { messageId: archive.messageId, sealed: toBase64(archive.sealed) } };
      const result = await sendChannelMessage({
        client,
        path: channel,
        body: { message: toBase64(message), ...copy },
      });
      return outcome(result.response?.status ?? 0, result.data?.id);
    },
    channelArchive: async (channel, before) => {
      const result = await listChannelArchive({
        client,
        path: channel,
        query: before === undefined ? {} : { before },
      });
      return required(result.data, "hand out the channel's archive").map(({ mlsGroupId, ...copy }) => ({
        ...copy,
        groupId: fromBase64(mlsGroupId),
        sealed: fromBase64(copy.sealed),
      }));
    },
  };
}

/** The messaging API on top of an instance's HTTP client. */
export function createMessagingApi(client: Client): MessagingApi {
  return {
    ...identityCalls(client),
    ...backupCalls(client),
    ...keyPackageCalls(client),
    ...groupCalls(client),
    ...fetchCalls(client),
    ...channelCalls(client),
  };
}
