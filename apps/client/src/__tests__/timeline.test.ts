import type { DecryptedMessage } from "../messaging/engine/sync";
import type { Payload } from "../messaging/payload";
import { buildTimeline, firstUnread, newestDeliveryId } from "../messaging/timeline";

let clock = 0;
const id = (n: number): string => `0195f0a2-1111-7000-8000-${String(n).padStart(12, "0")}`;
const at = (): string => new Date(Date.UTC(2026, 10, 10, 12, 0, clock++)).toISOString();
const from = (sender: string, n: number, payload: Payload): DecryptedMessage => ({
  id: id(n),
  sender,
  payload,
  sentAt: at(),
});
const ada = "@ada@test/laptop";
const adaPhone = "@ada@test/phone";
const grace = "@grace@test/phone";

describe("the conversation timeline", () => {
  it("shows text in order with replies, attachments and previews", () => {
    const attachment = {
      id: "u",
      name: "a.png",
      mimeType: "image/png",
      size: 1,
      scheme: "aes-256-gcm",
      key: "k",
      ciphertextSha256: "h",
    };
    const first = from(ada, 1, { kind: "text", text: "first" });
    const second = from(grace, 2, {
      kind: "text",
      text: "second",
      replyTo: id(1),
      attachments: [attachment],
      preview: { url: "https://e.com" },
    });
    // Delivered out of order: the timeline sorts by time.
    const messages = [second, first];
    const timeline = buildTimeline(messages);

    expect(timeline.map((item) => item.text)).toEqual(["first", "second"]);
    expect(timeline[0]).toMatchObject({
      sender: "@ada@test",
      edited: false,
      deleted: false,
      attachments: [],
      reactions: [],
    });
    expect(timeline[1]).toMatchObject({
      replyTo: id(1),
      attachments: [attachment],
      preview: { url: "https://e.com" },
    });
  });

  it("lets only the sender's user edit or delete, from any of their devices", () => {
    const timeline = buildTimeline([
      from(ada, 1, { kind: "text", text: "helo" }),
      from(grace, 2, { kind: "edit", target: id(1), text: "hacked" }),
      from(adaPhone, 3, { kind: "edit", target: id(1), text: "hello" }),
      from(ada, 4, { kind: "text", text: "oops" }),
      from(grace, 5, { kind: "delete", target: id(4) }),
      from(ada, 6, { kind: "text", text: "gone" }),
      from(ada, 7, { kind: "reaction", target: id(6), emoji: "👍", added: true }),
      from(ada, 8, { kind: "delete", target: id(6) }),
      from(ada, 9, { kind: "edit", target: id(6), text: "back?" }),
    ]);

    expect(timeline.map((item) => [item.text, item.edited, item.deleted])).toEqual([
      ["hello", true, false],
      ["oops", false, false],
      ["", false, true],
    ]);
    expect(timeline[2]?.reactions).toEqual([]);
  });

  it("collects reactions per emoji, once per user, and takes them back", () => {
    const timeline = buildTimeline([
      from(ada, 1, { kind: "text", text: "news" }),
      from(grace, 2, { kind: "reaction", target: id(1), emoji: "🎉", added: true }),
      from(adaPhone, 3, { kind: "reaction", target: id(1), emoji: "🎉", added: true }),
      from(ada, 4, { kind: "reaction", target: id(1), emoji: "🎉", added: true }),
      from(grace, 5, { kind: "reaction", target: id(1), emoji: "👍", added: true }),
      from(grace, 6, { kind: "reaction", target: id(1), emoji: "👍", added: false }),
      from(grace, 7, { kind: "reaction", target: id(1), emoji: "❤️", added: false }),
      from(grace, 8, { kind: "reaction", target: id(99), emoji: "👍", added: true }),
    ]);

    expect(timeline[0]?.reactions).toEqual([{ emoji: "🎉", users: ["@grace@test", "@ada@test"] }]);
  });

  it("keeps the first copy of a repeated message and names unknown senders as given", () => {
    const timeline = buildTimeline([
      from(ada, 1, { kind: "text", text: "once" }),
      from(ada, 1, { kind: "text", text: "twice" }),
      from("odd", 2, { kind: "text", text: "who" }),
    ]);

    expect(timeline.map((item) => item.text)).toEqual(["once", "who"]);
    expect(timeline[1]?.sender).toBe("odd");
  });

  it("finds where unread messages from others begin, by delivery ID", () => {
    const delivered = (message: DecryptedMessage, deliveryId: number): DecryptedMessage => ({
      ...message,
      deliveryId,
    });
    const messages = [
      delivered(from(ada, 21, { kind: "text", text: "mine" }), 5),
      delivered(from(grace, 22, { kind: "text", text: "old" }), 6),
      delivered(from(ada, 23, { kind: "text", text: "mine again" }), 7),
      delivered(from(grace, 24, { kind: "text", text: "new" }), 8),
      delivered(from(grace, 25, { kind: "reaction", target: id(24), emoji: "👍", added: true }), 9),
      from(grace, 26, { kind: "text", text: "saved before delivery IDs" }),
    ];
    const timeline = buildTimeline(messages);

    expect(timeline[0]?.deliveryId).toBe(5);
    expect(timeline.at(-1)?.deliveryId).toBeUndefined();
    expect(newestDeliveryId(messages)).toBe(9);
    expect(newestDeliveryId([])).toBe(0);
    expect(firstUnread(timeline, 6, "@ada@test")?.text).toBe("new");
    expect(firstUnread(timeline, 0, "@ada@test")?.text).toBe("old");
    expect(firstUnread(timeline, 9, "@ada@test")).toBeUndefined();
  });
});
