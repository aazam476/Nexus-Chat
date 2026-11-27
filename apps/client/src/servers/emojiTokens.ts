/**
 * Custom emoji in messages and reactions (spec §4.5): `<:name:id>`, which
 * every device draws as the emoji's picture. Typing `:` and part of a name
 * in the composer suggests the server's emoji.
 */

/** A custom emoji as messages refer to it. */
export interface EmojiRef {
  id: string;
  name: string;
}

/** The most suggestions shown at once. */
const suggestionLimit = 6;

const tokenPattern = /^<:([a-z0-9_]{2,32}):([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})>$/;

/** How a message or reaction writes `emoji`. */
export function emojiToken(emoji: EmojiRef): string {
  return `<:${emoji.name}:${emoji.id}>`;
}

/** The custom emoji a reaction names, or undefined for a Unicode emoji. */
export function parseEmojiToken(text: string): EmojiRef | undefined {
  const match = tokenPattern.exec(text);
  return match === null ? undefined : { name: match[1] ?? "", id: match[2] ?? "" };
}

/** The emoji name being typed after a trailing `:`, or undefined when there is none. */
export function emojiQuery(draft: string): string | undefined {
  return /(?:^|\s):([a-z0-9_]{1,32})$/.exec(draft)?.[1];
}

/** The emoji whose names start with `query`, then those that contain it. */
export function suggestEmoji<T extends EmojiRef>(query: string, emoji: readonly T[]): T[] {
  const starting = emoji.filter((candidate) => candidate.name.startsWith(query));
  const containing = emoji.filter(
    (candidate) => !candidate.name.startsWith(query) && candidate.name.includes(query),
  );
  return [...starting, ...containing].slice(0, suggestionLimit);
}

/** The draft with its trailing `:query` replaced by `emoji`. */
export function insertEmoji(draft: string, emoji: EmojiRef): string {
  return draft.replace(/:[a-z0-9_]{1,32}$/, `${emojiToken(emoji)} `);
}
