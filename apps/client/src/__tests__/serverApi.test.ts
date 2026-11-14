import { fromBase64, toBase64 } from "../messaging/engine/bytes";
import { createMessagingApi, ServerError } from "../messaging/engine/serverApi";
import { fakeServer } from "../testing/fakeServer";

const b64 = (...values: number[]): string => toBase64(new Uint8Array(values));
const conversation = "0195f0a2-1111-7000-8000-00000000c0de";
const user = "0195f0a2-1111-7000-8000-0000000000aa";

describe("the messaging API over HTTP", () => {
  it("turns base64 into bytes and back for identities and the key backup", async () => {
    const server = fakeServer();
    const api = createMessagingApi(server.client);
    server.answer(`GET /api/v1/users/${user}/identity`, {
      status: 200,
      body: { masterPublicKey: b64(1), selfSigningPublicKey: b64(2), selfSigningSignature: b64(3) },
    });
    server.answer("PUT /api/v1/identity", { status: 204 });
    server.answer("GET /api/v1/key-backup", {
      status: 200,
      body: {
        salt: b64(9),
        wrappedKeys: [{ method: "recovery-phrase", wrappedKey: b64(7) }],
        blobs: [{ name: "identity", size: 1, updatedAt: "2026-11-10T00:00:00Z" }],
      },
    });
    server.answer("PUT /api/v1/key-backup/wrapped-keys/password", { status: 204 });
    server.answer("PUT /api/v1/key-backup/blobs/identity", { status: 204 });
    server.answer("GET /api/v1/key-backup/blobs/identity", { status: 200, body: { data: b64(5) } });

    expect(await api.getIdentity(user)).toEqual({
      masterPublicKey: fromBase64(b64(1)),
      selfSigningPublicKey: fromBase64(b64(2)),
      selfSigningSignature: fromBase64(b64(3)),
    });
    expect(await api.getIdentity("someone-else")).toBeUndefined();
    await api.publishIdentity({
      masterPublicKey: new Uint8Array([1]),
      selfSigningPublicKey: new Uint8Array([2]),
      selfSigningSignature: new Uint8Array([3]),
    });
    expect(server.requests.at(-1)?.body).toEqual({
      masterPublicKey: b64(1),
      selfSigningPublicKey: b64(2),
      selfSigningSignature: b64(3),
    });
    const backup = await api.getBackup();
    expect(backup.salt).toEqual(new Uint8Array([9]));
    expect(backup.wrapped.get("recovery-phrase")).toEqual(new Uint8Array([7]));
    expect(backup.blobs).toEqual(["identity"]);
    await api.putWrappedKey("password", new Uint8Array([4]));
    await api.putBlob("identity", new Uint8Array([5]));
    expect(await api.getBlob("identity")).toEqual(new Uint8Array([5]));
    expect(await api.getBlob("missing")).toBeUndefined();
  });

  it("uploads and claims key packages", async () => {
    const server = fakeServer();
    const api = createMessagingApi(server.client);
    server.answer("GET /api/v1/key-packages", {
      status: 200,
      body: { oneTime: 3, hasLastResort: false, refill: true },
    });
    server.answer("POST /api/v1/key-packages", {
      status: 200,
      body: { oneTime: 4, hasLastResort: true, refill: true },
    });
    server.answer(`POST /api/v1/users/${user}/key-packages/claim`, {
      status: 200,
      body: [{ deviceId: "d1", keyPackage: b64(8), lastResort: false }],
    });

    expect(await api.keyPackageStock()).toEqual({ oneTime: 3, hasLastResort: false, refill: true });
    await api.uploadKeyPackages([new Uint8Array([1])], new Uint8Array([2]));
    expect(server.requests.at(-1)?.body).toEqual({ keyPackages: [b64(1)], lastResort: b64(2) });
    await api.uploadKeyPackages([new Uint8Array([1])]);
    expect(server.requests.at(-1)?.body).toEqual({ keyPackages: [b64(1)] });
    expect(await api.claimKeyPackages(user)).toEqual([{ deviceId: "d1", keyPackage: new Uint8Array([8]) }]);
  });

  it("reports commits and sends as accepted, stale or refused", async () => {
    const server = fakeServer();
    const api = createMessagingApi(server.client);
    const base = `/api/v1/conversations/${conversation}`;
    server.answer(`GET ${base}/mls`, { status: 200, body: { groupId: b64(1), epoch: 2, groupInfo: b64(3) } });
    server.answer(`POST ${base}/mls/commits`, { status: 200, body: { epoch: 3 } });
    server.answer(`POST ${base}/mls/messages`, { status: 201, body: { id: 12 } });
    server.answer(`GET ${base}/mls/messages`, {
      status: 200,
      body: [{ id: 1, kind: "commit", epoch: 0, data: b64(6), createdAt: "2026-11-10T00:00:00Z" }],
    });
    server.answer(`GET ${base}/archive`, {
      status: 200,
      body: [{ messageId: "m", epoch: 1, sealed: b64(4), createdAt: "2026-11-10T00:00:00Z" }],
    });

    expect(await api.groupState(conversation)).toEqual({
      groupId: new Uint8Array([1]),
      epoch: 2,
      groupInfo: new Uint8Array([3]),
    });
    const commit = { commit: new Uint8Array([1]), groupInfo: new Uint8Array([2]) };
    expect(await api.submitCommit(conversation, commit)).toEqual({ accepted: true });
    expect(await api.submitCommit(conversation, { ...commit, welcome: new Uint8Array([3]) })).toEqual({
      accepted: true,
    });
    expect(server.requests.at(-1)?.body).toEqual({ commit: b64(1), groupInfo: b64(2), welcome: b64(3) });
    expect(
      await api.sendMessage(conversation, new Uint8Array([1]), {
        messageId: "m",
        sealed: new Uint8Array([2]),
      }),
    ).toEqual({ accepted: true, id: 12 });
    expect((await api.messages(conversation, 0))[0]?.data).toEqual(new Uint8Array([6]));
    expect((await api.archive(conversation))[0]?.sealed).toEqual(new Uint8Array([4]));
    expect((await api.archive(conversation, "2026-11-10T00:00:00Z"))[0]?.messageId).toBe("m");

    server.answer(`POST ${base}/mls/commits`, {
      status: 409,
      body: { code: "epoch_conflict", message: "x" },
    });
    expect(await api.submitCommit(conversation, commit)).toEqual({ accepted: false, reason: "stale" });
    server.answer(`POST ${base}/mls/messages`, { status: 403, body: { code: "cannot_send", message: "x" } });
    expect(
      await api.sendMessage(conversation, new Uint8Array([1]), {
        messageId: "m",
        sealed: new Uint8Array([2]),
      }),
    ).toEqual({ accepted: false, reason: "refused" });
    server.answer(`GET ${base}/mls`, { status: 200, body: { groupId: b64(1), epoch: 0 } });
    expect(await api.groupState(conversation)).toEqual({ groupId: new Uint8Array([1]), epoch: 0 });
  });

  it("sends in server channels and reads their archive", async () => {
    const server = fakeServer();
    const api = createMessagingApi(server.client);
    const channel = { serverId: "s1", channelId: "c1" };
    const base = "/api/v1/servers/s1/channels/c1";
    server.answer(`POST ${base}/messages`, { status: 201, body: { id: 5 } });
    server.answer(`GET ${base}/archive`, {
      status: 200,
      body: [
        { messageId: "m", mlsGroupId: b64(9), epoch: 2, sealed: b64(4), createdAt: "2026-11-10T00:00:00Z" },
      ],
    });

    const archive = { messageId: "m", sealed: new Uint8Array([2]) };
    expect(await api.sendChannelMessage(channel, new Uint8Array([1]), archive)).toEqual({
      accepted: true,
      id: 5,
    });
    expect(server.requests.at(-1)?.body).toEqual({
      message: b64(1),
      archive: { messageId: "m", sealed: b64(2) },
    });
    expect(await api.sendChannelMessage(channel, new Uint8Array([1]))).toEqual({ accepted: true, id: 5 });
    expect(server.requests.at(-1)?.body).toEqual({ message: b64(1) });
    expect(await api.channelArchive(channel)).toEqual([
      {
        messageId: "m",
        groupId: new Uint8Array([9]),
        epoch: 2,
        sealed: new Uint8Array([4]),
        createdAt: "2026-11-10T00:00:00Z",
      },
    ]);
    expect(await api.channelArchive(channel, "2026-11-10T00:00:00Z")).toHaveLength(1);
    server.answer(`POST ${base}/messages`, { status: 403, body: { code: "cannot_send", message: "x" } });
    expect(await api.sendChannelMessage(channel, new Uint8Array([1]))).toEqual({
      accepted: false,
      reason: "refused",
    });
  });

  it("throws when the server refuses something the engine cannot go on from", async () => {
    const api = createMessagingApi(fakeServer().client);

    await expect(api.getBackup()).rejects.toThrow(ServerError);
    await expect(
      api.publishIdentity({
        masterPublicKey: new Uint8Array(),
        selfSigningPublicKey: new Uint8Array(),
        selfSigningSignature: new Uint8Array(),
      }),
    ).rejects.toThrow("publish the identity");
    await expect(api.putWrappedKey("passkey", new Uint8Array([1]))).rejects.toThrow(ServerError);
    await expect(api.putBlob("x", new Uint8Array([1]))).rejects.toThrow(ServerError);
    await expect(api.keyPackageStock()).rejects.toThrow(ServerError);
    await expect(api.uploadKeyPackages([])).rejects.toThrow(ServerError);
    await expect(api.claimKeyPackages(user)).rejects.toThrow(ServerError);
    await expect(api.groupState(conversation)).rejects.toThrow(ServerError);
    await expect(api.messages(conversation, 0)).rejects.toThrow(ServerError);
    await expect(api.archive(conversation)).rejects.toThrow(ServerError);
    await expect(api.channelArchive({ serverId: "s", channelId: "c" })).rejects.toThrow(ServerError);
    expect(
      await api.submitCommit(conversation, { commit: new Uint8Array(), groupInfo: new Uint8Array() }),
    ).toEqual({ accepted: false, reason: "refused" });
  });
});
