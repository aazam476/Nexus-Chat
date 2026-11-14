/**
 * What a server member may do, worked out on the device exactly as the
 * server does (docs/permissions.md). Members' apps need it for what the
 * server cannot see inside encrypted messages, such as whether a newcomer
 * may be sent a channel's history, or whether a reaction's sender was
 * allowed to react.
 */
import type { ChannelOverride, PermissionName, ServerRole } from "@nexus-chat/api-client";

/** Every permission, in the server's bit order. */
export const permissionNames: readonly PermissionName[] = [
  "ADMINISTRATOR",
  "MANAGE_SERVER",
  "MANAGE_ROLES",
  "MANAGE_CHANNELS",
  "MANAGE_EMOJI",
  "VIEW_AUDIT_LOG",
  "CREATE_INVITES",
  "KICK_MEMBERS",
  "BAN_MEMBERS",
  "TIMEOUT_MEMBERS",
  "VIEW_CHANNEL",
  "SEND_MESSAGES",
  "SEND_IN_THREADS",
  "CREATE_THREADS",
  "READ_HISTORY_BEFORE_JOIN",
  "ATTACH_FILES",
  "ADD_REACTIONS",
  "USE_CUSTOM_EMOJI",
  "MENTION_EVERYONE",
  "MANAGE_MESSAGES",
  "CONNECT_VOICE",
  "SPEAK",
  "VIDEO",
  "SCREEN_SHARE",
  "MUTE_MEMBERS",
  "DEAFEN_MEMBERS",
  "MOVE_MEMBERS",
];

/** A set of permissions. */
export type Permissions = ReadonlySet<PermissionName>;

const whileTimedOut: Permissions = new Set<PermissionName>(["VIEW_CHANNEL", "READ_HISTORY_BEFORE_JOIN"]);
const everything: Permissions = new Set(permissionNames);

/** One server member, as resolution needs them. */
export interface MemberView {
  userId: string;
  roleIds: readonly string[];
  /** Whether a timeout is running now. */
  timedOut?: boolean;
}

/** What decides a member's permissions: the owner and the roles. */
export interface ServerView {
  ownerId: string;
  /** Every role, `@everyone` included. */
  roles: readonly ServerRole[];
}

const keep = (granted: Permissions, kept: Permissions): Permissions =>
  new Set([...granted].filter((name) => kept.has(name)));

/** Denies come off, then allows go on. */
function applyOverride(
  granted: Permissions,
  deny: readonly PermissionName[],
  allow: readonly PermissionName[],
): Permissions {
  const result = new Set(granted);
  deny.forEach((name) => result.delete(name));
  allow.forEach((name) => result.add(name));
  return result;
}

/** Steps 1–3: the owner, `@everyone` and the member's roles, and administrators. */
function base(server: ServerView, member: MemberView): Permissions {
  if (member.userId === server.ownerId) {
    return everything;
  }
  const granted = new Set<PermissionName>();
  for (const role of server.roles) {
    if (role.everyone || member.roleIds.includes(role.id)) {
      role.permissions.forEach((name) => granted.add(name));
    }
  }
  return granted.has("ADMINISTRATOR") ? everything : granted;
}

/** What the member may do in the server, outside any channel. */
export function serverPermissions(server: ServerView, member: MemberView): Permissions {
  const granted = base(server, member);
  return granted !== everything && member.timedOut === true ? keep(granted, whileTimedOut) : granted;
}

/** What the member may do in a channel with these overrides. */
export function channelPermissions(
  server: ServerView,
  member: MemberView,
  overrides: readonly ChannelOverride[],
): Permissions {
  let granted = base(server, member);
  if (granted === everything) {
    return everything;
  }
  const everyoneId = server.roles.find((role) => role.everyone)?.id;
  const forRole = (override: ChannelOverride): boolean =>
    override.targetKind === "role" && override.targetId === everyoneId;
  const everyone = overrides.find(forRole);
  if (everyone !== undefined) {
    granted = applyOverride(granted, everyone.deny, everyone.allow);
  }
  const roles = overrides.filter(
    (override) => override.targetKind === "role" && member.roleIds.includes(override.targetId),
  );
  granted = applyOverride(
    granted,
    roles.flatMap((override) => override.deny),
    roles.flatMap((override) => override.allow),
  );
  const own = overrides.find(
    (override) => override.targetKind === "member" && override.targetId === member.userId,
  );
  if (own !== undefined) {
    granted = applyOverride(granted, own.deny, own.allow);
  }
  if (!granted.has("VIEW_CHANNEL")) {
    return new Set();
  }
  return member.timedOut === true ? keep(granted, whileTimedOut) : granted;
}
