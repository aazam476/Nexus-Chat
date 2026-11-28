import type { Messenger } from "../messaging/messenger";
import { audiencesOf } from "../servers/audiences";
import { banPurges, deliverDirective, moderatorDeletes, signingInstance } from "../testing/directives";
import { FakeInstance } from "../testing/fakeDelivery";
import { grace as graceUser } from "../testing/messengers";
import { audienceId, bookClub, general, messengersInServer } from "../testing/servers";

jest.setTimeout(60_000);

const audience = audiencesOf(bookClub(), [])[0];

async function texts(messenger: Messenger): Promise<string[]> {
  return (await messenger.messages(general)).flatMap((message) =>
    message.payload.kind === "text" ? [message.payload.text] : [],
  );
}

async function setup(): Promise<{
  instance: FakeInstance;
  key: Uint8Array;
  ada: Messenger;
  grace: Messenger;
}> {
  const instance = new FakeInstance();
  const key = await signingInstance(instance);
  const { ada, grace } = await messengersInServer(instance);
  return { instance, key, ada, grace };
}

describe("a moderator's deletions", () => {
  if (audience === undefined) {
    throw new Error("no audience");
  }

  it("drop a deleted message from every device, and from search", async () => {
    const { instance, key, ada, grace } = await setup();
    const spam = await grace.sendInChannel(audience, general, { kind: "text", text: "buy cheap watches" });
    await grace.sendInChannel(audience, general, { kind: "text", text: "sorry about that" });
    await ada.sync(audienceId);
    expect(await texts(ada)).toEqual(["buy cheap watches", "sorry about that"]);
    expect(await ada.search("watches")).toHaveLength(1);

    moderatorDeletes(instance, { audienceId, channelId: general }, { id: spam, senderId: graceUser.id }, key);
    await ada.sync(audienceId);
    await grace.sync(audienceId);
    expect(await texts(ada)).toEqual(["sorry about that"]);
    // The sender's own devices too.
    expect(await texts(grace)).toEqual(["sorry about that"]);
    expect(await ada.search("watches")).toEqual([]);
  });

  it("are ignored unless the instance signed them", async () => {
    const { instance, ada, grace } = await setup();
    const kept = await grace.sendInChannel(audience, general, { kind: "text", text: "keep me" });
    await ada.sync(audienceId);
    const forger = await signingInstance(new FakeInstance());
    deliverDirective(instance, audienceId, { type: "delete", messageId: kept, channelId: general }, forger);
    deliverDirective(instance, audienceId, { type: "delete", messageId: kept }, forger);
    await ada.sync(audienceId);
    expect(await texts(ada)).toEqual(["keep me"]);
  });

  it("purge what a banned member sent, even in the same sync", async () => {
    const { instance, key, ada, grace } = await setup();
    await ada.sendInChannel(audience, general, { kind: "text", text: "welcome" });
    const since = new Date(Date.now() - 1000).toISOString();
    await grace.sendInChannel(audience, general, { kind: "text", text: "raid one" });
    await ada.sync(audienceId);
    await grace.sendInChannel(audience, general, { kind: "text", text: "raid two" });
    banPurges(instance, { audienceId, channelId: general }, { userId: graceUser.id, since }, key);
    await ada.sync(audienceId);
    expect(await texts(ada)).toEqual(["welcome"]);
    expect(await ada.search("raid")).toEqual([]);
  });
});
