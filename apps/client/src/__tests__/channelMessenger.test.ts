import sodium from "libsodium-wrappers-sumo";

import type { Audience } from "../messaging/engine/channels";
import { type Messenger, type MessengerSetup } from "../messaging/messenger";
import { startMessenger } from "../messaging/messengerStart";
import { createMemoryDatabase } from "../storage/localDatabase";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const ada = { id: "user-ada", username: "ada" };
const grace = { id: "user-grace", username: "grace" };
const general = "0195f0e0-0000-7000-8000-00000000c001";
const audience: Audience = {
  serverId: "server-1",
  audienceId: "audience-1",
  channelIds: [general],
  mayReadHistory: () => true,
};

async function setupFor(instance: FakeInstance, user: typeof ada, deviceId: string): Promise<MessengerSetup> {
  await sodium.ready;
  return {
    api: instance.api(user.id, deviceId),
    database: createMemoryDatabase(),
    user,
    where: { domain: "test", deviceId },
    instanceKey: sodium.crypto_sign_keypair().publicKey,
  };
}

async function started(setup: MessengerSetup): Promise<Messenger> {
  const start = await startMessenger(setup);
  if (start.status !== "ready") {
    throw new Error("expected a ready messenger");
  }
  return start.messenger;
}

const texts = async (messenger: Messenger): Promise<string[]> =>
  (await messenger.messages(general))
    .map((message) => (message.payload.kind === "text" ? message.payload.text : message.payload.kind))
    .sort();

describe("the messenger in server channels", () => {
  it("joins an audience, sends, and gives a newcomer the channel's history", async () => {
    const instance = new FakeInstance();
    instance.addChannel(general, audience.audienceId);
    const adaMessenger = await started(await setupFor(instance, ada, "ada-laptop"));
    const graceSetup = await setupFor(instance, grace, "grace-phone");
    const graceMessenger = await started(graceSetup);
    const changed: string[] = [];
    graceMessenger.subscribe((id) => changed.push(id));

    await adaMessenger.openAudience(audience, [ada, grace]);
    await adaMessenger.sendInChannel(audience, general, { kind: "text", text: "before Grace" });
    await graceMessenger.openAudience(audience, [ada, grace]);
    expect(changed).toContain(general);
    // Ada's device sees Grace join and sends the history; Grace reads it.
    await adaMessenger.sync(audience.audienceId);
    await graceMessenger.sync(audience.audienceId);
    expect(await texts(graceMessenger)).toEqual(["before Grace"]);

    // Opening again reads nothing twice.
    await graceMessenger.openAudience(audience, [ada, grace]);
    expect(await texts(graceMessenger)).toEqual(["before Grace"]);

    // After a restart, the device still knows the group is an audience.
    const restarted = await started(graceSetup);
    await adaMessenger.sendInChannel(audience, general, { kind: "text", text: "after the restart" });
    await restarted.sync(audience.audienceId);
    expect(await texts(restarted)).toEqual(["after the restart", "before Grace"]);
  });
});
