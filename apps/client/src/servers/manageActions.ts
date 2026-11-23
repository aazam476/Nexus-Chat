/**
 * Changing a server (spec §4.4): its settings and icon, channels and their
 * order, roles and their order, members' roles, and channel permission
 * overrides. Each call resolves like the others in serverActions.ts.
 */
import {
  clearChannelOverride,
  createServerChannel,
  createServerRole,
  deleteServer,
  deleteServerChannel,
  deleteServerRole,
  giveServerRole,
  orderServerChannels,
  orderServerRoles,
  removeServerIcon,
  setChannelOverride,
  setServerIcon,
  takeServerRole,
  updateServer,
  updateServerChannel,
  updateServerRole,
  type ChannelPlacement,
  type Client,
  type NewServerChannel,
  type OverrideSpec,
  type OverrideTargetKind,
  type RoleSpec,
  type ServerSettings,
} from "@nexus-chat/api-client";

import { resultOf, type ServerResult } from "./serverActions";

/** Saves the server's name, description and media settings. */
export async function saveSettings(
  client: Client,
  serverId: string,
  settings: ServerSettings,
): Promise<ServerResult> {
  return resultOf(await updateServer({ client, path: { serverId }, body: settings }), serverId);
}

/** Deletes the server; only its owner may. */
export async function removeServer(client: Client, serverId: string): Promise<ServerResult> {
  return resultOf(await deleteServer({ client, path: { serverId } }), serverId);
}

/** Uploads an image as the server's icon. */
export async function uploadIcon(client: Client, serverId: string, image: Blob): Promise<ServerResult> {
  return resultOf(await setServerIcon({ client, path: { serverId }, body: image }), serverId);
}

/** Removes the server's icon. */
export async function removeIcon(client: Client, serverId: string): Promise<ServerResult> {
  return resultOf(await removeServerIcon({ client, path: { serverId } }), serverId);
}

/** Adds a channel or category; the result's ID is the new channel's. */
export async function addChannel(
  client: Client,
  serverId: string,
  channel: NewServerChannel,
): Promise<ServerResult> {
  return resultOf(await createServerChannel({ client, path: { serverId }, body: channel }));
}

/** Renames a channel and sets its topic. */
export async function editChannel(
  client: Client,
  where: { serverId: string; channelId: string },
  text: { name: string; topic: string },
): Promise<ServerResult> {
  return resultOf(await updateServerChannel({ client, path: where, body: text }), where.channelId);
}

/** Deletes a channel; a category's channels move out of it. */
export async function removeChannel(
  client: Client,
  serverId: string,
  channelId: string,
): Promise<ServerResult> {
  return resultOf(await deleteServerChannel({ client, path: { serverId, channelId } }), channelId);
}

/** Places every channel, in order. */
export async function placeChannels(
  client: Client,
  serverId: string,
  order: ChannelPlacement[],
): Promise<ServerResult> {
  return resultOf(await orderServerChannels({ client, path: { serverId }, body: order }), serverId);
}

/** Who a channel override is for. */
export interface OverrideTarget {
  serverId: string;
  channelId: string;
  targetKind: OverrideTargetKind;
  targetId: string;
}

/** Sets what a channel allows and denies for a role or member; nothing allowed or denied removes it. */
export async function saveOverride(
  client: Client,
  target: OverrideTarget,
  spec: OverrideSpec,
): Promise<ServerResult> {
  const answer =
    spec.allow.length === 0 && spec.deny.length === 0
      ? await clearChannelOverride({ client, path: target })
      : await setChannelOverride({ client, path: target, body: spec });
  return resultOf(answer, target.targetId);
}

/** Adds a role; the result's ID is the new role's. */
export async function addRole(client: Client, serverId: string, spec: RoleSpec): Promise<ServerResult> {
  return resultOf(await createServerRole({ client, path: { serverId }, body: spec }));
}

/** Changes a role's name, color and permissions. */
export async function editRole(
  client: Client,
  where: { serverId: string; roleId: string },
  spec: RoleSpec,
): Promise<ServerResult> {
  return resultOf(await updateServerRole({ client, path: where, body: spec }), where.roleId);
}

/** Deletes a role. */
export async function removeRole(client: Client, serverId: string, roleId: string): Promise<ServerResult> {
  return resultOf(await deleteServerRole({ client, path: { serverId, roleId } }), roleId);
}

/** Orders every role but `@everyone`, highest first. */
export async function placeRoles(client: Client, serverId: string, roleIds: string[]): Promise<ServerResult> {
  return resultOf(await orderServerRoles({ client, path: { serverId }, body: { roleIds } }), serverId);
}

/** Gives a member a role, or takes it away. */
export async function setMemberRole(
  client: Client,
  where: { serverId: string; userId: string; roleId: string },
  give: boolean,
): Promise<ServerResult> {
  const answer = give
    ? await giveServerRole({ client, path: where })
    : await takeServerRole({ client, path: where });
  return resultOf(answer, where.roleId);
}
