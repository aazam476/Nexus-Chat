import { decodeHistoryBundle } from "@nexus-chat/crypto";

import type { Audience } from "../messaging/engine/channels";
import { syncAudience } from "../messaging/engine/channels";
import { joinAudience, readChannelHistory, sendToChannel } from "../messaging/engine/channelSend";
import type { EngineContext } from "../messaging/engine/context";
import { loadMessages } from "../messaging/engine/history";
import { ConversationError } from "../messaging/engine/send";
import { device, instanceKeys, type TestDevice } from "../testing/engineDevices";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const general = "0195f0e0-0000-7000-8000-00000000c001";
const staff = "0195f0e0-0000-7000-8000-00000000c002";
const audienceId = "audience-1";

/** Grace and Alan may read #general's history; nobody but its first members #staff's. */
const audience: Audience = {
  serverId: "server-1",
  audienceId,
  channelIds: [general, staff],
  mayReadHistory: (_userId, channelId) => channelId === general,
};

const text = (body: string) => ({ kind: "text" as const, text: body });

async function texts(context: EngineContext, channelId: string): Promise<string[]> {
  return (await loadMessages(context, channelId)).map((message) =>
    message.payload.kind === "text" ? message.payload.text : message.payload.kind,
  );
}

async function setUp(): Promise<{
  instance: FakeInstance;
  ada: TestDevice;
  grace: TestDevice;
  alan: TestDevice;
}> {
  const instance = new FakeInstance();
  const { publicKey } = await instanceKeys();
  const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
  const grace = await device(instance, { name: "grace", deviceId: "grace-phone" }, publicKey);
  const alan = await device(instance, { name: "alan", deviceId: "alan-desk" }, publicKey);
  instance.addChannel(general, audienceId);
  instance.addChannel(staff, audienceId);
  return { instance, ada, grace, alan };
}

describe("server channels in the messaging engine", () => {
  it("send each channel's messages to their channel, and share history where it is allowed", async () => {
    const { instance, ada, grace } = await setUp();
    await joinAudience(ada.context, audience);
    await sendToChannel(ada.context, audience, general, text("welcome"));
    await sendToChannel(ada.context, audience, staff, text("staff only"));
    expect(await texts(ada.context, general)).toEqual(["welcome"]);

    // Grace joins; Ada's device sees her arrive and sends #general's keys.
    await joinAudience(grace.context, audience);
    await syncAudience(ada.context, audience);
    const synced = await syncAudience(grace.context, audience);
    expect([...synced.newHistory]).toEqual([general]);
    expect(await readChannelHistory(grace.context, audience, general)).toBe(1);
    expect(await texts(grace.context, general)).toEqual(["welcome"]);
    expect(await readChannelHistory(grace.context, audience, staff)).toBe(0);
    expect(await readChannelHistory(grace.context, audience, general)).toBe(0);

    // From now on, messages reach both, each in its channel.
    await sendToChannel(grace.context, audience, staff, text("hello staff"));
    const toAda = await syncAudience(ada.context, audience);
    expect(toAda.messages.map((message) => message.channelId)).toEqual([staff]);
    expect((await texts(ada.context, staff)).sort()).toEqual(["hello staff", "staff only"]);
    expect(instance.bundles).toBe(1);
  });

  it("drop a message whose server tag names another channel than the one inside", async () => {
    const { instance, ada, grace } = await setUp();
    await joinAudience(ada.context, audience);
    await joinAudience(grace.context, audience);
    await syncAudience(ada.context, audience);
    await syncAudience(grace.context, audience);

    await sendToChannel(ada.context, audience, general, text("for general"));
    const sent = instance.messages.get(audienceId)?.at(-1);
    if (sent !== undefined) {
      sent.channelId = staff;
    }
    const synced = await syncAudience(grace.context, audience);
    expect(synced.messages).toEqual([]);
    expect(await texts(grace.context, staff)).toEqual([]);
  });

  it("send a newcomer one bundle, not one from every member", async () => {
    const { instance, ada, grace, alan } = await setUp();
    await joinAudience(ada.context, audience);
    await joinAudience(grace.context, audience);
    await syncAudience(ada.context, audience);
    await syncAudience(grace.context, audience);
    await sendToChannel(ada.context, audience, general, text("before Alan"));
    await syncAudience(grace.context, audience);
    const before = instance.bundles;

    await joinAudience(alan.context, audience);
    await syncAudience(ada.context, audience);
    await syncAudience(grace.context, audience);
    expect(instance.bundles).toBe(before + 1);
    const bundle = instance.messages.get(audienceId)?.at(-1);
    expect(bundle?.channelId).toBe(general);

    await syncAudience(alan.context, audience);
    expect(await readChannelHistory(alan.context, audience, general)).toBe(1);
    expect(await texts(alan.context, general)).toEqual(["before Alan"]);
    expect(decodeHistoryBundle(new Uint8Array([1, 2, 3]))).toBeUndefined();
  });

  it("refuse to send before joining, or where the server refuses", async () => {
    const { ada } = await setUp();
    await expect(sendToChannel(ada.context, audience, general, text("early"))).rejects.toBeInstanceOf(
      ConversationError,
    );
    await joinAudience(ada.context, audience);
    const elsewhere = "0195f0e0-0000-7000-8000-00000000c009";
    await expect(sendToChannel(ada.context, audience, elsewhere, text("nowhere"))).rejects.toBeInstanceOf(
      ConversationError,
    );
  });
});
