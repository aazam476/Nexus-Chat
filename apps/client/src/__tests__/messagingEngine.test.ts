import { createIdentity } from "@nexus-chat/crypto";
import sodium from "libsodium-wrappers-sumo";

import { toBase64, utf8 } from "../messaging/engine/bytes";
import { heldMembers, refillKeyPackages, setUpDevice } from "../messaging/engine/device";
import { restoreWithPhrase, RestoreError, setUpIdentity } from "../messaging/engine/identity";
import {
  currentHistoryKey,
  ConversationError,
  sendPayload,
  startConversation,
} from "../messaging/engine/send";
import { syncConversation } from "../messaging/engine/sync";
import { acceptIdentityChange, credentialCheck, pinnedMasterKey } from "../messaging/engine/trust";
import { createMemoryDatabase } from "../storage/localDatabase";
import { device, instanceKeys, users } from "../testing/engineDevices";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const texts = (messages: Awaited<ReturnType<typeof syncConversation>>): string[] =>
  messages.map((message) => (message.payload.kind === "text" ? message.payload.text : message.payload.kind));

describe("the messaging engine", () => {
  it("runs a DM: creating the group, joining by Welcome, and messages both ways", async () => {
    const instance = new FakeInstance();
    const { publicKey } = await instanceKeys();
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    const grace = await device(instance, { name: "grace", deviceId: "grace-phone" }, publicKey);
    instance.addConversation("dm");

    await startConversation(ada.context, "dm", ["user-ada", "user-grace"]);
    await startConversation(grace.context, "dm", ["user-ada", "user-grace"]);
    const id = await sendPayload(ada.context, "dm", { kind: "text", text: "hello Grace" });

    const received = await syncConversation(grace.context, "dm");
    expect(texts(received)).toEqual(["hello Grace"]);
    expect(received[0]).toMatchObject({ id, sender: "@ada@test/ada-laptop" });
    await sendPayload(grace.context, "dm", { kind: "reaction", target: id, emoji: "👋", added: true });
    expect(texts(await syncConversation(ada.context, "dm"))).toEqual(["reaction"]);
    expect(await syncConversation(ada.context, "dm")).toEqual([]);
    expect(await currentHistoryKey(ada.context, "dm")).toEqual(await currentHistoryKey(grace.context, "dm"));
    expect(await currentHistoryKey(ada.context, "none")).toBeUndefined();
  });

  it("brings a user's second device in: restore from the phrase, then an external join", async () => {
    const instance = new FakeInstance();
    const { publicKey } = await instanceKeys();
    const laptop = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    const grace = await device(instance, { name: "grace", deviceId: "grace-phone" }, publicKey);
    instance.addConversation("dm");
    await startConversation(laptop.context, "dm", ["user-ada", "user-grace"]);
    await startConversation(grace.context, "dm", ["user-ada", "user-grace"]);

    expect(laptop.phrase?.split(" ")).toHaveLength(24);
    const phone = await device(
      instance,
      { name: "ada", deviceId: "ada-phone", phrase: laptop.phrase },
      publicKey,
    );
    expect(toBase64(phone.identity.selfSigningKey.publicKey)).toBe(
      toBase64(laptop.identity.selfSigningKey.publicKey),
    );
    await startConversation(phone.context, "dm", ["user-ada", "user-grace"]);

    // Grace's device was at the old epoch: her send is stale, catches up, and goes out.
    await sendPayload(grace.context, "dm", { kind: "text", text: "hi both" });
    expect(texts(await syncConversation(phone.context, "dm"))).toEqual(["hi both"]);
    expect(texts(await syncConversation(laptop.context, "dm"))).toEqual(["hi both"]);
  });

  it("removes a member who left, on a directive signed by the instance", async () => {
    const instance = new FakeInstance();
    const keys = await instanceKeys();
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, keys.publicKey);
    const grace = await device(instance, { name: "grace", deviceId: "grace-phone" }, keys.publicKey);
    const alan = await device(instance, { name: "alan", deviceId: "alan-desk" }, keys.publicKey);
    instance.addConversation("group");
    const everyone = ["user-ada", "user-grace", "user-alan"];
    for (const member of [ada, grace, alan]) {
      await startConversation(member.context, "group", everyone);
    }

    const directive = utf8(JSON.stringify({ type: "remove", userId: "user-alan", reason: "left" }));
    const signature = sodium.crypto_sign_detached(
      new Uint8Array([...utf8("nexus-chat membership directive v1\n"), ...directive]),
      keys.privateKey,
    );
    const forged = sodium.crypto_sign_detached(directive, keys.privateKey);
    for (const signed of [forged, signature]) {
      instance.deliver("group", {
        kind: "directive",
        epoch: 1,
        data: utf8(JSON.stringify({ directive: toBase64(directive), signature: toBase64(signed) })),
      });
    }
    await syncConversation(ada.context, "group");
    await syncConversation(grace.context, "group");
    await sendPayload(grace.context, "group", { kind: "text", text: "just us" });

    expect(texts(await syncConversation(ada.context, "group"))).toEqual(["just us"]);
    expect(await syncConversation(alan.context, "group")).toEqual([]);
  });

  it("refuses to restore without a backup or with the wrong phrase", async () => {
    const instance = new FakeInstance();
    const api = instance.api("user-ada", "d1");
    const database = createMemoryDatabase();
    await expect(restoreWithPhrase(api, database, "user-ada", "x")).rejects.toThrow(RestoreError);

    const { publicKey } = await instanceKeys();
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    expect(ada.phrase).toBeDefined();
    const second = await setUpIdentity(instance.api("user-ada", "d2"), createMemoryDatabase(), "user-ada");
    expect(second.status).toBe("needs-restore");
    const wrong = `${"abandon ".repeat(23)}art`;
    await expect(
      restoreWithPhrase(instance.api("user-ada", "d2"), createMemoryDatabase(), "user-ada", wrong),
    ).rejects.toThrow();
  });

  it("keeps device keys and key packages, and refills only when asked", async () => {
    const instance = new FakeInstance();
    const { publicKey } = await instanceKeys();
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    const again = await setUpDevice(ada.context.database, ada.identity, {
      user: "@ada@test",
      deviceId: "ada-laptop",
    });

    expect(again.signatureKeys.publicKey).toEqual(ada.context.device.signatureKeys.publicKey);
    expect(await refillKeyPackages(ada.context.api, ada.context.database, ada.context.device)).toBe(0);
    expect(await heldMembers(ada.context.database)).toHaveLength(51);
  });

  it("pins each user's master key and flags a change", async () => {
    const instance = new FakeInstance();
    const { publicKey } = await instanceKeys();
    await device(instance, { name: "grace", deviceId: "grace-phone" }, publicKey);
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    const changes: string[] = [];
    const { api, database } = ada.context;
    const check = credentialCheck(
      api,
      database,
      (user) => users[user],
      (userId) => changes.push(userId),
    );
    instance.addConversation("dm");
    await startConversation({ ...ada.context, check }, "dm", ["user-ada", "user-grace"]);
    expect(await pinnedMasterKey(database, "user-grace")).toEqual(
      instance.identities.get("user-grace")?.masterPublicKey,
    );

    // Grace's account now publishes another identity (she started fresh):
    // her old device no longer checks out until Ada accepts the change.
    const fresh = await createIdentity();
    instance.identities.set("user-grace", {
      masterPublicKey: fresh.masterKey.publicKey,
      selfSigningPublicKey: fresh.selfSigningKey.publicKey,
      selfSigningSignature: fresh.selfSigningSignature,
    });
    instance.addConversation("dm2");
    await expect(
      startConversation({ ...ada.context, check }, "dm2", ["user-ada", "user-grace"]),
    ).rejects.toThrow();
    expect(changes).toContain("user-grace");

    await acceptIdentityChange(api, database, "user-grace");
    expect(await pinnedMasterKey(database, "user-grace")).toEqual(fresh.masterKey.publicKey);
    expect(await pinnedMasterKey(database, "nobody")).toBeUndefined();
    await acceptIdentityChange(api, database, "nobody");
  });

  it("will not send before the device is in the conversation", async () => {
    const instance = new FakeInstance();
    const { publicKey } = await instanceKeys();
    const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
    instance.addConversation("dm");

    await expect(sendPayload(ada.context, "dm", { kind: "text", text: "x" })).rejects.toThrow(
      ConversationError,
    );
  });
});
