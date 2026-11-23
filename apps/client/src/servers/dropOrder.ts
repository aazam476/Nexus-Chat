/**
 * Where a dragged row lands, and the channel or role order that results.
 * The list is flat as the screen shows it: for channels, the channels
 * outside any category, then each category followed by its channels; for
 * roles, every role but `@everyone`, highest first.
 */
import type { ChannelPlacement, ServerChannel, ServerDetails } from "@nexus-chat/api-client";

import { placementsOf } from "./channelOrder";
import { channelSections } from "./channelTree";
import { mayManageRole, rolesHighestFirst } from "./hierarchy";

/** Where a row sits in its list, from its layout. */
export interface RowLayout {
  y: number;
  height: number;
}

/**
 * Where the row at `from`, dragged by `translationY`, is dropped: its index
 * among the other rows, counting the rows whose middle its middle passed.
 */
export function dropIndex(layouts: readonly RowLayout[], from: number, translationY: number): number {
  const dragged = layouts[from];
  if (dragged === undefined) {
    return from;
  }
  const middle = dragged.y + dragged.height / 2 + translationY;
  return layouts.filter((layout, index) => index !== from && layout.y + layout.height / 2 < middle).length;
}

/** The channels in the order the list shows them. */
export function channelRows(channels: readonly ServerChannel[]): ServerChannel[] {
  return channelSections(channels).flatMap(({ category, channels: inside }) =>
    category === undefined ? inside : [category, ...inside],
  );
}

/** The flat rows with a channel inserted at `to`, its category the nearest one above it. */
function withChannelAt(rows: readonly ServerChannel[], channel: ServerChannel, to: number): ServerChannel[] {
  const others = rows.filter((row) => row.id !== channel.id);
  const placed = [...others.slice(0, to), channel, ...others.slice(to)];
  let parentId: string | undefined;
  return placed.map((row, position) => {
    if (row.kind === "category") {
      parentId = row.id;
      return { ...row, position };
    }
    const moved: ServerChannel = { ...row, position };
    delete moved.parentId;
    return parentId === undefined ? moved : { ...moved, parentId };
  });
}

/** The flat rows with a category and its channels moved to before the category at `to`. */
function withCategoryAt(
  rows: readonly ServerChannel[],
  category: ServerChannel,
  to: number,
): ServerChannel[] {
  const others = rows.filter((row) => row.id !== category.id);
  const before = others.slice(0, to).filter((row) => row.kind === "category").length;
  const categories = others.filter((row) => row.kind === "category");
  categories.splice(before, 0, category);
  return rows.map((row) => (row.kind === "category" ? { ...row, position: categories.indexOf(row) } : row));
}

/** The placements once channel `id` is dropped at `to`; undefined when nothing changes. */
export function droppedChannel(
  channels: readonly ServerChannel[],
  id: string,
  to: number,
): ChannelPlacement[] | undefined {
  const rows = channelRows(channels);
  const channel = rows.find((row) => row.id === id);
  if (channel === undefined) {
    return undefined;
  }
  const after =
    channel.kind === "category" ? withCategoryAt(rows, channel, to) : withChannelAt(rows, channel, to);
  const before = placementsOf(channelSections(channels));
  const placements = placementsOf(channelSections(after));
  return JSON.stringify(placements) === JSON.stringify(before) ? undefined : placements;
}

/** The channels arranged as `placements` say, for showing a move before the server confirms it. */
export function placedChannels(
  channels: readonly ServerChannel[],
  placements: readonly ChannelPlacement[],
): ServerChannel[] {
  return channels.map((channel) => {
    const at = placements.findIndex((placement) => placement.id === channel.id);
    const placement = placements[at];
    if (placement === undefined) {
      return channel;
    }
    const placed: ServerChannel = { ...channel, position: at };
    delete placed.parentId;
    return placement.parentId === undefined ? placed : { ...placed, parentId: placement.parentId };
  });
}

/**
 * The role IDs, highest first, once `roleId` is dropped at `to` among the
 * other roles; undefined when nothing changes or the move would pass a role
 * the user may not manage.
 */
export function droppedRole(
  details: ServerDetails,
  userId: string,
  roleId: string,
  to: number,
): string[] | undefined {
  const order = rolesHighestFirst(details).filter((role) => !role.everyone);
  const role = order.find((candidate) => candidate.id === roleId);
  if (role === undefined || !mayManageRole(details, userId, role)) {
    return undefined;
  }
  const others = order.filter((candidate) => candidate.id !== roleId);
  const after = [...others.slice(0, to), role, ...others.slice(to)];
  const fixed = order.every(
    (candidate, index) => mayManageRole(details, userId, candidate) || after[index] === candidate,
  );
  const changed = after.some((candidate, index) => order[index] !== candidate);
  return fixed && changed ? after.map((candidate) => candidate.id) : undefined;
}

/** The server's roles positioned as `ids` (highest first) say, `@everyone` staying lowest. */
export function placedRoles(details: ServerDetails, ids: readonly string[]): ServerDetails {
  const roles = details.roles.map((role) => {
    const at = ids.indexOf(role.id);
    return at < 0 ? role : { ...role, position: ids.length - at };
  });
  return { ...details, roles };
}
