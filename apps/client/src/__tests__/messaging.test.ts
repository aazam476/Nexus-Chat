import fc from "fast-check";

import { parseInline, parseMarkdown, plainText, type Block, type Inline } from "../messaging/markdown";
import { decodePayload, encodePayload, payloadLimits, type Payload } from "../messaging/payload";

const text = (value: string): Inline => ({ type: "text", text: value });
const id = "0195f0a2-1111-7000-8000-000000000001";

describe("Markdown", () => {
  it("reads every construct of the subset", () => {
    expect(parseInline("**bold** *it* _it_ ~~gone~~ ||secret|| `code`")).toEqual([
      { type: "bold", children: [text("bold")] },
      text(" "),
      { type: "italic", children: [text("it")] },
      text(" "),
      { type: "italic", children: [text("it")] },
      text(" "),
      { type: "strike", children: [text("gone")] },
      text(" "),
      { type: "spoiler", children: [text("secret")] },
      text(" "),
      { type: "code", text: "code" },
    ]);
    expect(parseInline("**bold _and italic_**")).toEqual([
      { type: "bold", children: [text("bold "), { type: "italic", children: [text("and italic")] }] },
    ]);
    // A marker opens only before text and closes only after it.
    expect(parseInline("a ** b ** c *d *e*")).toEqual([
      text("a ** b ** c "),
      { type: "italic", children: [text("d *e")] },
    ]);
  });

  it("makes links only to safe addresses", () => {
    expect(parseInline("[site](https://example.com) see https://a.example/x")).toEqual([
      { type: "link", url: "https://example.com", children: [text("site")] },
      text(" see "),
      { type: "link", url: "https://a.example/x", children: [text("https://a.example/x")] },
    ]);
    expect(parseInline("[x](javascript:alert(1))")).toEqual([text("[x](javascript:alert(1))")]);
    expect(parseInline("[x](mailto:a@example.com)")).toEqual([
      { type: "link", url: "mailto:a@example.com", children: [text("x")] },
    ]);
    expect(parseInline("[open](https://example.com")).toEqual([text("[open](https://example.com")]);
    expect(parseInline("[two\nlines](https://example.com)")[0]).toEqual(text("[two"));
    expect(parseInline("nohttps://example.com")).toEqual([text("nohttps://example.com")]);
  });

  it("reads mentions", () => {
    expect(parseInline(`<@${id}> <@&role1> <#chan> @everyone @here @hereafter`)).toEqual([
      { type: "mention", kind: "user", id },
      text(" "),
      { type: "mention", kind: "role", id: "role1" },
      text(" "),
      { type: "mention", kind: "channel", id: "chan" },
      text(" "),
      { type: "mention", kind: "everyone" },
      text(" "),
      { type: "mention", kind: "here" },
      text(" @hereafter"),
    ]);
    expect(parseInline("<@>")).toEqual([text("<@>")]);
  });

  it("keeps unmatched and escaped markers as text", () => {
    expect(parseInline("2 * 3 = 6 and **open")).toEqual([text("2 * 3 = 6 and **open")]);
    expect(parseInline("\\*not italic\\* \\x")).toEqual([text("*not italic* \\x")]);
    expect(parseInline("`` and ****")).toEqual([text("`` and ****")]);
    expect(parseInline("one\ntwo")).toEqual([text("one"), { type: "break" }, text("two")]);
  });

  it("stops nesting at the depth limit", () => {
    const deep = "**_".repeat(10) + "x" + "_**".repeat(10);
    let depth = 0;
    let nodes: Inline[] = parseInline(deep);
    while (nodes[0] !== undefined && "children" in nodes[0]) {
      nodes = nodes[0].children;
      depth += 1;
    }
    expect(depth).toBeLessThanOrEqual(6);
  });

  it("reads blocks: paragraphs, code and quotes", () => {
    const blocks: Block[] = parseMarkdown(
      "hello\n```ts\nconst x = 1;\n**not bold**\n```\n> quoted\n> more\nafter\n```\nunclosed",
    );
    expect(blocks).toEqual([
      { type: "paragraph", children: [text("hello")] },
      { type: "code", language: "ts", text: "const x = 1;\n**not bold**" },
      { type: "quote", children: [text("quoted"), { type: "break" }, text("more")] },
      {
        type: "paragraph",
        children: [text("after"), { type: "break" }, text("```"), { type: "break" }, text("unclosed")],
      },
    ]);
    expect(parseMarkdown("```\nplain\n```\n>")).toEqual([
      { type: "code", text: "plain" },
      { type: "quote", children: [] },
    ]);
    expect(parseMarkdown("")).toEqual([{ type: "paragraph", children: [] }]);
  });

  it("gives plain text for screen readers and notifications", () => {
    expect(plainText(parseInline(`**hi** <@${id}> @here \`x\`\n[a](https://e.com)`))).toBe(
      `hi @${id} @here x\na`,
    );
  });

  it("never throws, and leaves text without markers as it is", () => {
    fc.assert(
      fc.property(fc.string({ maxLength: 300 }), (source) => {
        expect(() => parseMarkdown(source)).not.toThrow();
      }),
    );
    const plain = fc
      .string({ maxLength: 200 })
      .filter((value) => !/[\\*_~|`[\]()<>#@:\n]/.test(value) && value !== "");
    fc.assert(
      fc.property(plain, (source) => {
        expect(parseInline(source)).toEqual([text(source)]);
      }),
    );
  });
});

describe("message payloads", () => {
  const attachment = {
    id: "upload-1",
    name: "photo.webp",
    mimeType: "image/webp",
    size: 1200,
    width: 640,
    height: 480,
    scheme: "aes-256-gcm",
    key: "a2V5",
    nonce: "bm9uY2U=",
    ciphertextSha256: "aGFzaA==",
    thumbnail: {
      id: "upload-2",
      mimeType: "image/webp",
      scheme: "aes-256-gcm",
      key: "a2V5",
      ciphertextSha256: "aGFzaA==",
    },
  };

  it("round-trip every kind", () => {
    const payloads: Payload[] = [
      { kind: "text", text: "hello" },
      { kind: "text", text: "", attachments: [attachment] },
      {
        kind: "text",
        text: "re",
        replyTo: id,
        preview: { url: "https://e.com", title: "E", image: attachment },
      },
      { kind: "edit", target: id, text: "fixed" },
      { kind: "delete", target: id },
      { kind: "reaction", target: id, emoji: "👍", added: true },
    ];
    for (const payload of payloads) {
      expect(decodePayload(encodePayload(payload))).toEqual(payload);
    }
  });

  it("drop anything malformed, unknown or too large", () => {
    const raw = (value: unknown): Uint8Array => new TextEncoder().encode(JSON.stringify(value));
    const refused = [
      new TextEncoder().encode("not json"),
      raw(null),
      raw({ v: 2, kind: "text", text: "future" }),
      raw({ v: 1, kind: "poll" }),
      raw({ v: 1, kind: "text" }),
      raw({ v: 1, kind: "text", text: "x".repeat(payloadLimits.textLength + 1) }),
      raw({ v: 1, kind: "text", text: "x", replyTo: "not an id" }),
      raw({ v: 1, kind: "text", text: "x", attachments: "no" }),
      raw({ v: 1, kind: "text", text: "x", attachments: [{ id: 1 }] }),
      raw({ v: 1, kind: "text", text: "x", attachments: [null] }),
      raw({ v: 1, kind: "text", text: "x", attachments: [{ ...attachment, thumbnail: { id: "t" } }] }),
      raw({ v: 1, kind: "text", text: "x", attachments: [{ ...attachment, width: -1 }] }),
      raw({ v: 1, kind: "text", text: "x", attachments: Array.from({ length: 11 }, () => attachment) }),
      raw({ v: 1, kind: "text", text: "x", preview: { title: "no url" } }),
      raw({ v: 1, kind: "text", text: "x", preview: "nope" }),
      raw({ v: 1, kind: "edit", target: id }),
      raw({ v: 1, kind: "delete", target: "x" }),
      raw({ v: 1, kind: "reaction", target: id, emoji: "👍" }),
    ];
    for (const bytes of refused) {
      expect(decodePayload(bytes)).toBeUndefined();
    }
  });

  it("never throws on arbitrary bytes", () => {
    fc.assert(
      fc.property(fc.uint8Array({ maxLength: 200 }), (bytes) => {
        expect(() => decodePayload(bytes)).not.toThrow();
      }),
    );
  });
});
