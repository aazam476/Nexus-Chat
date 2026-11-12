import { createIdentity } from "@nexus-chat/crypto";
import sodium from "libsodium-wrappers-sumo";

import {
  confirmRecoveryPhrase,
  restoreMessenger,
  startMessenger,
  type Messenger,
  type MessengerSetup,
} from "../messaging/messenger";
import { createMemoryDatabase } from "../storage/localDatabase";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const adaUser = { id: "user-ada", username: "ada" };
const graceUser = { id: "user-grace", username: "grace" };

async function setupFor(
  instance: FakeInstance,
  user: { id: string; username: string },
  deviceId: string,
): Promise<MessengerSetup> {
  await sodium.ready;
  return {
    api: instance.api(user.id, deviceId),
    database: createMemoryDatabase(),
    user,
    where: { domain: "test", deviceId },
    instanceKey: sodium.crypto_sign_keypair().publicKey,
  };
}

async function started(setup: MessengerSetup): Promise<{ messenger: Messenger; phrase?: string }> {
  const start = await startMessenger(setup);
  if (start.status !== "ready") {
    throw new Error("expected a ready messenger");
  }
  return {
    messenger: start.messenger,
    ...(start.recoveryPhrase === undefined ? {} : { phrase: start.recoveryPhrase }),
  };
}

const texts = (messages: Awaited<ReturnType<Messenger["messages"]>>): string[] =>
  messages.map((message) => (message.payload.kind === "text" ? message.payload.text : message.payload.kind));

describe("the messenger", () => {
  it("opens a DM, sends, syncs and keeps both sides' messages", async () => {
    const instance = new FakeInstance();
    const ada = await started(await setupFor(instance, adaUser, "ada-laptop"));
    const grace = await started(await setupFor(instance, graceUser, "grace-phone"));
    instance.addConversation("dm");
    const changed: string[] = [];
    const stop = grace.messenger.subscribe((id) => changed.push(id));

    await ada.messenger.open("dm", [adaUser, graceUser]);
    await grace.messenger.open("dm", [adaUser, graceUser]);
    await ada.messenger.send("dm", { kind: "text", text: "hello" });
    await grace.messenger.sync("dm");
    stop();
    await grace.messenger.sync("dm");

    expect(texts(await grace.messenger.messages("dm"))).toEqual(["hello"]);
    // Both sides know the message's delivery ID, which read state counts by.
    const [received] = await grace.messenger.messages("dm");
    const [sent] = await ada.messenger.messages("dm");
    expect(received?.deliveryId).toEqual(expect.any(Number));
    expect(sent?.deliveryId).toBe(received?.deliveryId);
    expect(await ada.messenger.messages("dm")).toMatchObject([
      { sender: "@ada@test/ada-laptop", payload: { kind: "text", text: "hello" } },
    ]);
    expect(changed).toEqual(["dm", "dm"]);
    expect(ada.phrase?.split(" ")).toHaveLength(24);
    expect(ada.messenger.identityChanges()).toEqual([]);
  });

  it("asks a second device to restore, then works after the phrase", async () => {
    const instance = new FakeInstance();
    const laptopSetup = await setupFor(instance, adaUser, "ada-laptop");
    const laptop = await started(laptopSetup);
    // Until confirmed, the phrase is shown again after a restart.
    expect((await started(laptopSetup)).phrase).toBe(laptop.phrase);
    await confirmRecoveryPhrase(laptopSetup.database);
    expect((await started(laptopSetup)).phrase).toBeUndefined();
    const phoneSetup = await setupFor(instance, adaUser, "ada-phone");

    expect(await startMessenger(phoneSetup)).toEqual({ status: "needs-restore" });
    const phone = await restoreMessenger(phoneSetup, laptop.phrase ?? "");
    instance.addConversation("notes");
    await phone.open("notes", [adaUser]);
    await phone.send("notes", { kind: "text", text: "to self" });
    expect(texts(await phone.messages("notes"))).toEqual(["to self"]);
    expect((await started(phoneSetup)).phrase).toBeUndefined();
  });

  it("keeps working on a conversation after a failed step", async () => {
    const instance = new FakeInstance();
    const ada = await started(await setupFor(instance, adaUser, "ada-laptop"));
    instance.addConversation("dm");

    await expect(ada.messenger.send("dm", { kind: "text", text: "too early" })).rejects.toThrow();
    await ada.messenger.open("dm", [adaUser]);
    await expect(ada.messenger.send("dm", { kind: "text", text: "now" })).resolves.toEqual(
      expect.any(String),
    );
  });

  it("flags a contact whose identity changed until it is accepted", async () => {
    const instance = new FakeInstance();
    const ada = await started(await setupFor(instance, adaUser, "ada-laptop"));
    await started(await setupFor(instance, graceUser, "grace-phone"));
    instance.addConversation("dm");
    await ada.messenger.open("dm", [adaUser, graceUser]);

    const fresh = await createIdentity();
    instance.identities.set(graceUser.id, {
      masterPublicKey: fresh.masterKey.publicKey,
      selfSigningPublicKey: fresh.selfSigningKey.publicKey,
      selfSigningSignature: fresh.selfSigningSignature,
    });
    instance.addConversation("dm2");
    await expect(ada.messenger.open("dm2", [adaUser, graceUser])).rejects.toThrow();
    expect(ada.messenger.identityChanges()).toEqual([graceUser.id]);
    await ada.messenger.acceptIdentityChange(graceUser.id);
    expect(ada.messenger.identityChanges()).toEqual([]);
  });
});
