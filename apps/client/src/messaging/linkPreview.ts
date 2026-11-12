/**
 * Link previews on the sender's device (spec §4.5, docs/e2ee-design.md §8):
 * the first link in a message is fetched once, here, and its title,
 * description and image travel encrypted inside the message. Recipients
 * never fetch the link.
 */
import { parseMarkdown, type Inline } from "./markdown";

/** What a page shows in a preview. */
export interface PreviewPage {
  title?: string;
  description?: string;
  /** Absolute, http or https. */
  imageUrl?: string;
}

const maximumTitle = 300;
const maximumDescription = 1000;

function linksIn(nodes: Inline[]): string[] {
  return nodes.flatMap((node) => {
    if (node.type === "link") {
      return [node.url];
    }
    return "children" in node ? linksIn(node.children) : [];
  });
}

/** The http and https links in a message's Markdown, in order. */
export function linksOf(text: string): string[] {
  return parseMarkdown(text)
    .flatMap((block) => (block.type === "code" ? [] : linksIn(block.children)))
    .filter((url) => /^https?:\/\//i.test(url));
}

/** The first http or https link in a message's Markdown, if any. */
export function firstLink(text: string): string | undefined {
  return linksOf(text)[0];
}

const entities: Record<string, string> = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", "#39": "'" };

function decode(text: string): string {
  return text.replace(/&(amp|lt|gt|quot|apos|#39);/g, (_, name: string) => entities[name] ?? "").trim();
}

function shorten(text: string, limit: number): string {
  const characters = Array.from(text);
  return characters.length <= limit ? text : `${characters.slice(0, limit - 1).join("")}…`;
}

/** The attributes of one tag, lowercased names. */
function attributesOf(tag: string): Map<string, string> {
  const attributes = new Map<string, string>();
  for (const match of tag.matchAll(/([a-zA-Z:-]+)\s*=\s*("([^"]*)"|'([^']*)')/g)) {
    attributes.set((match[1] ?? "").toLowerCase(), decode(match[3] ?? match[4] ?? ""));
  }
  return attributes;
}

/** The head's meta tags, first one of each name winning. */
function metaTags(head: string): Map<string, string> {
  const tags = new Map<string, string>();
  for (const match of head.matchAll(/<meta\b[^>]*>/gi)) {
    const attributes = attributesOf(match[0]);
    const key = (attributes.get("property") ?? attributes.get("name") ?? "").toLowerCase();
    if (key !== "" && !tags.has(key)) {
      tags.set(key, attributes.get("content") ?? "");
    }
  }
  return tags;
}

const firstOf = (...values: (string | undefined)[]): string | undefined =>
  values.find((value) => value !== undefined && value !== "");

/** Reads a page's preview from its HTML; only the head counts. */
export function parsePreviewHtml(html: string, pageUrl: string): PreviewPage {
  const head = html.split(/<body\b/i)[0] ?? "";
  const tags = metaTags(head);
  const title = firstOf(
    tags.get("og:title"),
    tags.get("twitter:title"),
    decode(/<title[^>]*>([\s\S]*?)<\/title>/i.exec(head)?.[1] ?? ""),
  );
  const description = firstOf(
    tags.get("og:description"),
    tags.get("twitter:description"),
    tags.get("description"),
  );
  const image = firstOf(tags.get("og:image"), tags.get("og:image:url"), tags.get("twitter:image"));
  let imageUrl: string | undefined;
  try {
    const resolved = image === undefined ? undefined : new URL(image, pageUrl);
    imageUrl = resolved !== undefined && /^https?:$/.test(resolved.protocol) ? resolved.href : undefined;
  } catch {
    imageUrl = undefined;
  }
  return {
    ...(title === undefined ? {} : { title: shorten(title, maximumTitle) }),
    ...(description === undefined ? {} : { description: shorten(description, maximumDescription) }),
    ...(imageUrl === undefined ? {} : { imageUrl }),
  };
}
