import type { DecryptedMessage } from "../messaging/engine/sync";
import type { Payload } from "../messaging/payload";
import { indexMessages } from "../search/indexer";
import { searchMessages } from "../search/query";
import { chunkKeys, readChunk } from "../search/searchIndex";
import { fold, terms } from "../search/tokens";
import { createMemoryDatabase, type LocalDatabase } from "../storage/localDatabase";

const now = new Date("2026-11-08T12:00:00Z");
const dm = "dm-1";
const channel = "channel-1";
const ada = "@ada@chat.example.com";
const grace = "@grace@chat.example.com";

let counter = 0;
function message(
  payload: Payload,
  options: { sender?: string; sentAt?: string; id?: string; thread?: string } = {},
): DecryptedMessage {
  counter += 1;
  return {
    id: options.id ?? `0195f0e0-0000-7000-8000-${String(counter).padStart(12, "0")}`,
    sender: `${options.sender ?? ada}/laptop`,
    payload,
    sentAt: options.sentAt ?? "2026-11-07T10:00:00Z",
    ...(options.thread === undefined ? {} : { thread: options.thread }),
  };
}

const text = (body: string, attachments = false): Payload => ({
  kind: "text",
  text: body,
  ...(attachments
    ? {
        attachments: [
          {
            id: "a",
            name: "report.pdf",
            mimeType: "application/pdf",
            size: 1,
            scheme: "x",
            key: "k",
            ciphertextSha256: "s",
          },
        ],
      }
    : {}),
});

async function indexed(messages: DecryptedMessage[], conversationId = dm): Promise<LocalDatabase> {
  const database = createMemoryDatabase();
  await indexMessages(database, conversationId, messages, { now });
  return database;
}

const ids = (hits: { id: string }[]) => hits.map((hit) => hit.id);

describe("search terms", () => {
  it("fold case and accents, and split words in any script", () => {
    expect(fold("Café ÜBER naïve")).toBe("cafe uber naive");
    expect(terms("Hello, world! It's 2026.")).toEqual(["hello", "world", "it", "s", "2026"]);
    expect(terms("مرحبا بالعالم")).toEqual(["مرحبا", "بالعالم"]);
    expect(terms("Привет, мир")).toEqual(["привет", "мир"]);
  });

  it("split scripts without spaces into pairs of characters", () => {
    expect(terms("東京都")).toEqual(["東京", "京都"]);
    expect(terms("猫")).toEqual(["猫"]);
    expect(terms("iPhone東京")).toEqual(["iphone", "東京"]);
    expect(terms("東京iPhone")).toEqual(["東京", "iphone"]);
  });

  it("cut very long words", () => {
    expect(terms("a".repeat(100))[0]).toHaveLength(40);
  });
});

describe("the search index", () => {
  it("finds messages by the start of every word, newest first", async () => {
    const old = message(text("Quarterly report draft"), { sentAt: "2026-11-01T09:00:00Z" });
    const recent = message(text("The report is final"), { sentAt: "2026-11-05T09:00:00Z" });
    const other = message(text("Lunch?"));
    const database = await indexed([recent, old, other]);

    expect(ids(await searchMessages(database, "rep"))).toEqual([recent.id, old.id]);
    expect(ids(await searchMessages(database, "REPORT quart"))).toEqual([old.id]);
    expect(await searchMessages(database, "report dinner")).toEqual([]);
    expect(ids(await searchMessages(database, ""))).toEqual([other.id, recent.id, old.id]);
    const [hit] = await searchMessages(database, "lunch");
    expect(hit).toMatchObject({ conversationId: dm, sender: ada, text: "Lunch?", hasAttachments: false });
  });

  it("filters by sender, conversation, attachments and date", async () => {
    const fromAda = message(text("plans for friday", true), { sentAt: "2026-10-30T09:00:00Z" });
    const fromGrace = message(text("plans for saturday"), { sender: grace, sentAt: "2026-11-03T09:00:00Z" });
    const database = await indexed([fromAda, fromGrace]);
    const inChannel = message(text("plans in the channel"), { thread: "root" });
    await indexMessages(database, channel, [inChannel], { now });

    expect(ids(await searchMessages(database, "plans", { from: grace }))).toEqual([fromGrace.id]);
    expect(ids(await searchMessages(database, "plans", { in: [channel] }))).toEqual([inChannel.id]);
    expect(ids(await searchMessages(database, "plans", { hasAttachments: true }))).toEqual([fromAda.id]);
    expect(ids(await searchMessages(database, "plans", { after: "2026-11-01T00:00:00Z", in: [dm] }))).toEqual(
      [fromGrace.id],
    );
    expect(ids(await searchMessages(database, "plans", { before: "2026-11-01T00:00:00Z" }))).toEqual([
      fromAda.id,
    ]);
    expect((await searchMessages(database, "channel"))[0]?.thread).toBe("root");
  });

  it("follows edits and deletions by the sender, and ignores others'", async () => {
    const original = message(text("meet at noon"));
    const database = await indexed([original]);
    await indexMessages(database, dm, [message({ kind: "edit", target: original.id, text: "meet at one" })], {
      now,
    });
    expect(await searchMessages(database, "noon")).toEqual([]);
    expect(ids(await searchMessages(database, "one"))).toEqual([original.id]);

    // Someone else cannot change or remove it; reactions and unknown targets do nothing.
    await indexMessages(
      database,
      dm,
      [
        message({ kind: "delete", target: original.id }, { sender: grace }),
        message({ kind: "reaction", target: original.id, emoji: "👍", added: true }),
        message({ kind: "delete", target: "unknown" }),
      ],
      { now },
    );
    expect(ids(await searchMessages(database, "meet"))).toEqual([original.id]);

    await indexMessages(database, dm, [message({ kind: "delete", target: original.id })], { now });
    expect(await searchMessages(database, "meet")).toEqual([]);
    expect(await chunkKeys(database)).toEqual([]); // an empty chunk is removed
  });

  it("keeps the recent window unless asked for everything", async () => {
    const ancient = message(text("ancient history"), { sentAt: "2026-01-01T00:00:00Z" });
    const database = await indexed([ancient]);
    expect(await searchMessages(database, "ancient")).toEqual([]);
    await indexMessages(database, dm, [ancient], { now, everything: true });
    expect(ids(await searchMessages(database, "ancient"))).toEqual([ancient.id]);
  });

  it("keeps chunks by conversation and month, safe for any word", async () => {
    const database = await indexed([
      message(text("__proto__ constructor"), { sentAt: "2026-10-30T09:00:00Z" }),
      message(text("hasOwnProperty"), { sentAt: "2026-11-02T09:00:00Z" }),
    ]);
    expect(await chunkKeys(database)).toEqual([`${dm}|2026-10`, `${dm}|2026-11`]);
    const chunk = await readChunk(database, `${dm}|2026-10`);
    expect([...chunk.terms.keys()]).toEqual(["proto", "constructor"]);
    expect(ids(await searchMessages(database, "__proto__"))).toHaveLength(1);
    expect(ids(await searchMessages(database, "hasown"))).toHaveLength(1);
  });
});
