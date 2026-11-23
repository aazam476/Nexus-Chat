/**
 * Rearranging channels: moving one up or down among the channels next to
 * it, or into another category. The server takes the whole order at once
 * (every channel, in order within each category), so each move gives the
 * full list of placements.
 */
import type { ChannelPlacement, ServerChannel } from "@nexus-chat/api-client";

import { channelSections } from "./channelTree";

/** The channels' placements in the order they are shown. */
export function placementsOf(sections: ReturnType<typeof channelSections>): ChannelPlacement[] {
  const placements: ChannelPlacement[] = [];
  for (const { category, channels } of sections) {
    if (category !== undefined) {
      placements.push({ id: category.id });
    }
    for (const channel of channels) {
      placements.push(
        category === undefined ? { id: channel.id } : { id: channel.id, parentId: category.id },
      );
    }
  }
  return placements;
}

/** The sections with `id` swapped with its neighbor `by` places away, if it has one. */
function swapped(
  sections: ReturnType<typeof channelSections>,
  id: string,
  by: -1 | 1,
): ReturnType<typeof channelSections> | undefined {
  const categories = sections.filter((section) => section.category !== undefined);
  const at = categories.findIndex((section) => section.category?.id === id);
  if (at >= 0) {
    const other = categories[at + by];
    if (other === undefined) {
      return undefined;
    }
    const loose = sections.filter((section) => section.category === undefined);
    const reordered = [...categories];
    reordered[at + by] = categories[at] ?? other;
    reordered[at] = other;
    return [...loose, ...reordered];
  }
  return sections.map((section) => {
    const index = section.channels.findIndex((channel) => channel.id === id);
    const neighbor = section.channels[index + by];
    if (index < 0 || neighbor === undefined) {
      return section;
    }
    const channels = [...section.channels];
    channels[index + by] = section.channels[index] ?? neighbor;
    channels[index] = neighbor;
    return { ...section, channels };
  });
}

/**
 * Every channel's placement after moving `id` one place up (-1) or down
 * (1) among its siblings, or undefined when it is already at that end.
 */
export function moved(
  channels: readonly ServerChannel[],
  id: string,
  by: -1 | 1,
): ChannelPlacement[] | undefined {
  const before = placementsOf(channelSections(channels));
  const after = swapped(channelSections(channels), id, by);
  if (after === undefined) {
    return undefined;
  }
  const placements = placementsOf(after);
  return JSON.stringify(placements) === JSON.stringify(before) ? undefined : placements;
}

/** Every channel's placement after moving `id` to the end of category `parentId` (or out of any). */
export function movedInto(
  channels: readonly ServerChannel[],
  id: string,
  parentId: string | undefined,
): ChannelPlacement[] {
  const channel = channels.find((candidate) => candidate.id === id);
  const others = channels.filter((candidate) => candidate.id !== id);
  if (channel === undefined || channel.kind === "category") {
    return placementsOf(channelSections(channels));
  }
  const siblings = others.filter(
    (candidate) => candidate.kind !== "category" && candidate.parentId === parentId,
  );
  const last = Math.max(-1, ...siblings.map((sibling) => sibling.position)) + 1;
  const placed: ServerChannel = { ...channel, position: last };
  delete placed.parentId;
  if (parentId !== undefined) {
    placed.parentId = parentId;
  }
  return placementsOf(channelSections([...others, placed]));
}
