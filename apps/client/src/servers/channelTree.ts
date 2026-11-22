/**
 * A server's channels in the order members see them: channels outside any
 * category first, then each category with its channels, each by position.
 */
import type { ServerChannel } from "@nexus-chat/api-client";

/** A category and its channels; `category` is undefined for the channels outside any. */
export interface ChannelSection {
  category: ServerChannel | undefined;
  channels: ServerChannel[];
}

const byPosition = (a: ServerChannel, b: ServerChannel): number =>
  a.position - b.position || a.id.localeCompare(b.id);

/** The channels grouped into sections, in order. Empty categories are kept, so they can be filled. */
export function channelSections(channels: readonly ServerChannel[]): ChannelSection[] {
  const categories = channels.filter((channel) => channel.kind === "category").sort(byPosition);
  const known = new Set(categories.map((category) => category.id));
  const inside = (parent: string | undefined): ServerChannel[] =>
    channels
      .filter(
        (channel) =>
          channel.kind !== "category" &&
          (parent === undefined
            ? channel.parentId === undefined || !known.has(channel.parentId)
            : channel.parentId === parent),
      )
      .sort(byPosition);
  const loose = inside(undefined);
  return [
    ...(loose.length === 0 ? [] : [{ category: undefined, channels: loose }]),
    ...categories.map((category) => ({ category, channels: inside(category.id) })),
  ];
}
