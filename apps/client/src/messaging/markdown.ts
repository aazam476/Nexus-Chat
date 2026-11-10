/**
 * The Markdown subset of spec §4.5: bold (`**`), italic (`*` or `_`),
 * strikethrough (`~~`), spoilers (`||`), inline code, fenced code blocks,
 * block quotes (`> `), links (`[text](https://…)` and bare https links) and
 * mentions (`<@user>`, `<@&role>`, `<#channel>`, `@everyone`, `@here`).
 *
 * Messages come from other people, so the parser never throws, never
 * nests deeper than `maximumDepth`, keeps unmatched markers as plain text,
 * and only makes links to http, https and mailto addresses. It builds a
 * tree; the message view decides how each node looks.
 */

/** Inline content. */
export type Inline =
  | { type: "text"; text: string }
  | { type: "break" }
  | { type: "bold" | "italic" | "strike" | "spoiler"; children: Inline[] }
  | { type: "code"; text: string }
  | { type: "link"; url: string; children: Inline[] }
  | { type: "mention"; kind: "user" | "role" | "channel"; id: string }
  | { type: "mention"; kind: "everyone" | "here" };

/** A block of a message. */
export type Block =
  | { type: "paragraph"; children: Inline[] }
  | { type: "code"; language?: string; text: string }
  | { type: "quote"; children: Inline[] };

const maximumDepth = 5;

/** Paired markers, longest first so `**` wins over `*`. */
const pairs: { marker: string; type: "bold" | "italic" | "strike" | "spoiler" }[] = [
  { marker: "**", type: "bold" },
  { marker: "~~", type: "strike" },
  { marker: "||", type: "spoiler" },
  { marker: "*", type: "italic" },
  { marker: "_", type: "italic" },
];

const escapable = new Set(["\\", "*", "_", "~", "|", "`", "[", "]", "(", ")", "<", ">", "#", "@"]);
const safeLink = /^(https?:\/\/[^\s<>]+|mailto:[^\s<>]+)$/i;
const bareLink = /^https?:\/\/[^\s<>]+/i;
const mentionPattern = /^<(@&|@|#)([0-9a-zA-Z-]{1,64})>/;

/** Adds text to the end of `out`, joining it to text already there. */
function pushText(out: Inline[], text: string): void {
  const last = out.at(-1);
  if (last?.type === "text") {
    last.text += text;
  } else if (text !== "") {
    out.push({ type: "text", text });
  }
}

/** A link `[text](url)` at `start`, or undefined. */
function linkAt(source: string, start: number, depth: number): { node: Inline; end: number } | undefined {
  const close = source.indexOf("](", start);
  const end = close === -1 ? -1 : source.indexOf(")", close + 2);
  if (close === -1 || end === -1) {
    return undefined;
  }
  const url = source.slice(close + 2, end);
  if (!safeLink.test(url) || source.slice(start + 1, close).includes("\n")) {
    return undefined;
  }
  return {
    node: { type: "link", url, children: parseInline(source.slice(start + 1, close), depth + 1) },
    end: end + 1,
  };
}

/** A mention at `start`, or undefined. */
function mentionAt(source: string, start: number): { node: Inline; end: number } | undefined {
  for (const kind of ["everyone", "here"] as const) {
    const word = `@${kind}`;
    if (source.startsWith(word, start) && !/[\w]/.test(source[start + word.length] ?? "")) {
      return { node: { type: "mention", kind }, end: start + word.length };
    }
  }
  const match = mentionPattern.exec(source.slice(start));
  if (match === null) {
    return undefined;
  }
  const kinds = { "@": "user", "@&": "role", "#": "channel" } as const;
  const sigil = match[1] as keyof typeof kinds;
  return {
    node: { type: "mention", kind: kinds[sigil], id: match[2] ?? "" },
    end: start + match[0].length,
  };
}

const isSpace = (character: string): boolean => character === "" || /\s/.test(character);

/**
 * Where `marker` closes a span opened at `from`: the next occurrence with
 * text right before it (so `2 * 3` is not italic), or -1.
 */
function closingMarker(source: string, marker: string, from: number): number {
  for (let close = source.indexOf(marker, from); close !== -1; close = source.indexOf(marker, close + 1)) {
    if (close > from && !isSpace(source.charAt(close - 1))) {
      return close;
    }
  }
  return -1;
}

/**
 * A paired span such as `**bold**` at `start`, or undefined. It opens only
 * before text, and a run like `**` is never read as two single markers.
 */
function spanAt(source: string, start: number, depth: number): { node: Inline; end: number } | undefined {
  for (const { marker, type } of pairs) {
    const next = source.charAt(start + marker.length);
    if (!source.startsWith(marker, start) || isSpace(next) || (marker.length === 1 && next === marker)) {
      continue;
    }
    const close = closingMarker(source, marker, start + marker.length);
    if (close !== -1) {
      const inner = source.slice(start + marker.length, close);
      return { node: { type, children: parseInline(inner, depth + 1) }, end: close + marker.length };
    }
  }
  return undefined;
}

/** Whatever special thing starts at `start`, or undefined for plain text. */
function tokenAt(source: string, start: number, depth: number): { node: Inline; end: number } | undefined {
  const character = source[start];
  if (character === "`") {
    const close = source.indexOf("`", start + 1);
    return close > start + 1
      ? { node: { type: "code", text: source.slice(start + 1, close) }, end: close + 1 }
      : undefined;
  }
  if (character === "[") {
    return linkAt(source, start, depth);
  }
  if (character === "<" || character === "@") {
    return mentionAt(source, start);
  }
  const link = bareLink.exec(source.slice(start));
  if (link !== null && (start === 0 || /\s/.test(source[start - 1] ?? ""))) {
    return {
      node: { type: "link", url: link[0], children: [{ type: "text", text: link[0] }] },
      end: start + link[0].length,
    };
  }
  return depth < maximumDepth ? spanAt(source, start, depth) : undefined;
}

/** Parses one line or span of inline content. */
export function parseInline(source: string, depth = 0): Inline[] {
  const out: Inline[] = [];
  let index = 0;
  while (index < source.length) {
    const character = source.charAt(index);
    if (character === "\\" && escapable.has(source[index + 1] ?? "")) {
      pushText(out, source.charAt(index + 1));
      index += 2;
      continue;
    }
    if (character === "\n") {
      out.push({ type: "break" });
      index += 1;
      continue;
    }
    const token = tokenAt(source, index, depth);
    if (token === undefined) {
      pushText(out, character);
      index += 1;
    } else {
      out.push(token.node);
      index = token.end;
    }
  }
  return out;
}

const isQuoteLine = (line: string): boolean => line.startsWith("> ") || line === ">";

/** The fenced code block starting at `start`, and the index of its closing line. */
function codeBlockAt(lines: string[], start: number): { block: Block; end: number } | undefined {
  const opening = lines[start] ?? "";
  const close = opening.startsWith("```")
    ? lines.findIndex((line, at) => at > start && line.trim() === "```")
    : -1;
  if (close === -1) {
    return undefined;
  }
  const language = opening.slice(3).trim();
  const text = lines.slice(start + 1, close).join("\n");
  return { block: { type: "code", ...(language === "" ? {} : { language }), text }, end: close };
}

/** The quote starting at `start`, and the index of its last line. */
function quoteAt(lines: string[], start: number): { block: Block; end: number } {
  let end = start;
  while (end + 1 < lines.length && isQuoteLine(lines[end + 1] ?? "")) {
    end += 1;
  }
  const quoted = lines.slice(start, end + 1).map((line) => line.slice(2));
  return { block: { type: "quote", children: parseInline(quoted.join("\n")) }, end };
}

/** Parses a whole message into blocks. */
export function parseMarkdown(source: string): Block[] {
  const blocks: Block[] = [];
  const lines = source.split("\n");
  let paragraph: string[] = [];
  const flush = (): void => {
    if (paragraph.length > 0) {
      blocks.push({ type: "paragraph", children: parseInline(paragraph.join("\n")) });
      paragraph = [];
    }
  };
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index] ?? "";
    const special = codeBlockAt(lines, index) ?? (isQuoteLine(line) ? quoteAt(lines, index) : undefined);
    if (special === undefined) {
      paragraph.push(line);
      continue;
    }
    flush();
    blocks.push(special.block);
    index = special.end;
  }
  flush();
  return blocks;
}

/** The text a screen reader or notification shows: markers removed, mentions as written. */
export function plainText(nodes: Inline[]): string {
  return nodes
    .map((node) => {
      switch (node.type) {
        case "text":
        case "code":
          return node.text;
        case "break":
          return "\n";
        case "mention":
          return "id" in node ? `@${node.id}` : `@${node.kind}`;
        default:
          return plainText(node.children);
      }
    })
    .join("");
}
