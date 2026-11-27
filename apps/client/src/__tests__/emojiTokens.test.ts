import { parseMarkdown, plainText } from "../messaging/markdown";
import { emojiQuery, emojiToken, insertEmoji, parseEmojiToken, suggestEmoji } from "../servers/emojiTokens";

const wave = { id: "0195f0e0-0000-7000-8000-00000000e011", name: "wave" };
const list = [
  { id: "0195f0e0-0000-7000-8000-00000000e012", name: "big_wave" },
  wave,
  { id: "0195f0e0-0000-7000-8000-00000000e013", name: "party" },
];

describe("custom emoji in text", () => {
  it("are written as tokens and read back", () => {
    expect(emojiToken(wave)).toBe(`<:wave:${wave.id}>`);
    expect(parseEmojiToken(emojiToken(wave))).toEqual(wave);
    expect(parseEmojiToken("👍")).toBeUndefined();
    expect(parseEmojiToken("<:Wave:not-an-id>")).toBeUndefined();
  });

  it("are parsed in messages, and named in plain text", () => {
    const [block] = parseMarkdown(`hi <:wave:${wave.id}> there <:x:${wave.id}>`);
    expect(block).toEqual({
      type: "paragraph",
      children: [
        { type: "text", text: "hi " },
        { type: "emoji", name: "wave", id: wave.id },
        { type: "text", text: ` there <:x:${wave.id}>` },
      ],
    });
    expect(plainText(block?.type === "paragraph" ? block.children : [])).toBe(
      `hi :wave: there <:x:${wave.id}>`,
    );
    // Mentions still parse next to them.
    expect(parseMarkdown("<@abc>")[0]).toMatchObject({
      children: [{ type: "mention", kind: "user", id: "abc" }],
    });
  });

  it("are suggested while a name is typed after a colon", () => {
    expect(emojiQuery("hello :wa")).toBe("wa");
    expect(emojiQuery(":p")).toBe("p");
    expect(emojiQuery("time 10:30")).toBeUndefined();
    expect(emojiQuery("hello :")).toBeUndefined();
    expect(suggestEmoji("wa", list).map((emoji) => emoji.name)).toEqual(["wave", "big_wave"]);
    expect(suggestEmoji("zzz", list)).toEqual([]);
    expect(insertEmoji("hello :wa", wave)).toBe(`hello <:wave:${wave.id}> `);
  });
});
