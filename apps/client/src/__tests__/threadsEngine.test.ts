import type { Audience } from "../messaging/engine/channels";
import { syncAudience } from "../messaging/engine/channels";
import {
  joinAudience,
  readChannelHistory,
  readThreadHistory,
  sendToChannel,
} from "../messaging/engine/channelSend";
import type { EngineContext } from "../messaging/engine/context";
import { loadMessages } from "../messaging/engine/history";
import { decodePayload, encodePayload } from "../messaging/payload";
import { buildTimeline, channelItems, threadItems } from "../messaging/timeline";
import { device, instanceKeys, type TestDevice } from "../testing/engineDevices";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const general = "0195f0e0-0000-7000-8000-00000000c001";
const audienceId = "audience-1";
const audience: Audience = {
  serverId: "server-1",
  audienceId,
  channelIds: [general],
  mayReadHistory: () => true,
};
const text = (body: string) => ({ kind: "text" as const, text: body });

async function setUp(): Promise<{ instance: FakeInstance; ada: TestDevice; grace: TestDevice }> {
  const instance = new FakeInstance();
  const { publicKey } = await instanceKeys();
  const ada = await device(instance, { name: "ada", deviceId: "ada-laptop" }, publicKey);
  const grace = await device(instance, { name: "grace", deviceId: "grace-phone" }, publicKey);
  instance.addChannel(general, audienceId);
  await joinAudience(ada.context, audience);
  await joinAudience(grace.context, audience);
  await syncAudience(ada.context, audience);
  await syncAudience(grace.context, audience);
  return { instance, ada, grace };
}

async function threads(context: EngineContext): Promise<(string | undefined)[]> {
  return (await loadMessages(context, general)).map((message) => message.thread);
}

describe("threads in the messaging engine", () => {
  it("carry their root inside the encryption, checked against the server's tag", async () => {
    const { instance, ada, grace } = await setUp();
    const root = await sendToChannel(ada.context, audience, { channelId: general }, text("root"));
    await sendToChannel(ada.context, audience, { channelId: general, thread: root }, text("reply"));
    expect(instance.messages.get(audienceId)?.at(-1)?.threadRootId).toBe(root);

    await syncAudience(grace.context, audience);
    expect(await threads(grace.context)).toEqual([undefined, root]);

    // A reply whose tag was changed or removed on the way is dropped.
    await sendToChannel(ada.context, audience, { channelId: general, thread: root }, text("moved"));
    const moved = instance.messages.get(audienceId)?.at(-1);
    if (moved !== undefined) {
      delete moved.threadRootId;
    }
    await sendToChannel(ada.context, audience, { channelId: general }, text("plain"));
    const tagged = instance.messages.get(audienceId)?.at(-1);
    if (tagged !== undefined) {
      tagged.threadRootId = root;
    }
    const synced = await syncAudience(grace.context, audience);
    expect(synced.messages).toEqual([]);
  });

  it("read a thread's replies from its own archive, not the channel's", async () => {
    const { ada, grace } = await setUp();
    const root = await sendToChannel(ada.context, audience, { channelId: general }, text("root"));
    for (const body of ["one", "two", "three"]) {
      await sendToChannel(ada.context, audience, { channelId: general, thread: root }, text(body));
    }
    // Grace's device reads the archives instead of the live copies.
    expect(await readChannelHistory(grace.context, audience, general)).toBe(1);
    expect(await threads(grace.context)).toEqual([undefined]);
    expect(await readThreadHistory(grace.context, audience, general, root)).toBe(3);
    expect(await readThreadHistory(grace.context, audience, general, root)).toBe(0);
    expect((await threads(grace.context)).filter((thread) => thread === root)).toHaveLength(3);
  });
});

describe("threads in the timeline", () => {
  const message = (id: string, minute: number, thread?: string) => ({
    id,
    sender: "@ada@chat.example.com/laptop",
    payload: text(id),
    sentAt: `2026-11-08T12:0${String(minute)}:00Z`,
    ...(thread === undefined ? {} : { thread }),
  });
  const timeline = buildTimeline([
    message("a", 0),
    message("b", 1),
    message("a1", 2, "a"),
    message("a2", 3, "a"),
  ]);

  it("leave replies out of the channel and count them on their root", () => {
    expect(channelItems(timeline).map((item) => [item.id, item.replies])).toEqual([
      ["a", { count: 2, lastAt: "2026-11-08T12:03:00Z" }],
      ["b", undefined],
    ]);
  });

  it("show a thread as its root, then its replies", () => {
    expect(threadItems(timeline, "a").map((item) => item.id)).toEqual(["a", "a1", "a2"]);
    expect(threadItems(timeline, "b").map((item) => item.id)).toEqual(["b"]);
  });

  it("keep the root inside the payload", () => {
    const id = "0195f0e0-0000-7000-8000-00000000d001";
    const thread = "0195f0e0-0000-7000-8000-00000000d002";
    expect(decodePayload(encodePayload({ id, payload: text("x"), thread }))).toEqual({
      id,
      payload: text("x"),
      thread,
    });
    expect(decodePayload(encodePayload({ id, payload: text("x"), thread: "not-an-id" }))).toEqual({
      id,
      payload: text("x"),
    });
  });
});
