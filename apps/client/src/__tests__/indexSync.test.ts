import sodium from "libsodium-wrappers-sumo";

import { type MessengerSetup } from "../messaging/messenger";
import { restoreMessenger, startMessenger } from "../messaging/messengerStart";
import { createMemoryDatabase } from "../storage/localDatabase";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const adaUser = { id: "user-ada", username: "ada" };

async function setupFor(instance: FakeInstance, deviceId: string): Promise<MessengerSetup> {
  await sodium.ready;
  return {
    api: instance.api(adaUser.id, deviceId),
    database: createMemoryDatabase(),
    user: adaUser,
    where: { domain: "test", deviceId },
    instanceKey: sodium.crypto_sign_keypair().publicKey,
  };
}

describe("sharing the search index between a user's devices", () => {
  it("lets a new device search what another indexed, without the messages", async () => {
    const instance = new FakeInstance();
    const start = await startMessenger(await setupFor(instance, "ada-laptop"));
    if (start.status !== "ready" || start.recoveryPhrase === undefined) {
      throw new Error("expected a new account");
    }
    const laptop = start.messenger;
    instance.addConversation("notes");
    await laptop.open("notes", [adaUser]);
    await laptop.send("notes", { kind: "text", text: "the boarding pass is in my email" });
    await laptop.send("notes", { kind: "text", text: "passport renewal next month" });

    expect(await laptop.syncSearchIndex()).toEqual({ fetched: 0, shared: 1 });
    const blobs = [...(instance.backups.get(adaUser.id)?.blobs.keys() ?? [])].filter((name) =>
      name.startsWith("search."),
    );
    expect(blobs).toEqual([`search.notes.${new Date().toISOString().slice(0, 7)}`]);
    // Nothing changed since, so nothing goes up again.
    expect(await laptop.syncSearchIndex()).toEqual({ fetched: 1, shared: 0 });

    // A phone restored from the recovery phrase finds them once it fetches
    // the shared index, without reading any history.
    const phone = await restoreMessenger(await setupFor(instance, "ada-phone"), start.recoveryPhrase);
    expect(await phone.search("pass")).toEqual([]);
    expect(await phone.syncSearchIndex()).toEqual({ fetched: 1, shared: 0 });
    expect(await phone.search("pass")).toHaveLength(2);
    expect((await phone.search("boarding")).map((hit) => hit.text)).toEqual([
      "the boarding pass is in my email",
    ]);
    expect(await phone.messages("notes")).toEqual([]);
  });
});
