/**
 * Channel overrides for the screens: the permissions that mean something
 * in a channel, who an override is for, and an override's three-way state
 * for each permission.
 */
import type {
  ChannelOverride,
  OverrideSpec,
  OverrideTargetKind,
  PermissionName,
  ServerDetails,
  ServerMember,
} from "@nexus-chat/api-client";

/** The permissions a channel override can change; the rest apply to the whole server. */
export const channelLevelPermissions: readonly PermissionName[] = [
  "VIEW_CHANNEL",
  "MANAGE_CHANNELS",
  "SEND_MESSAGES",
  "SEND_IN_THREADS",
  "CREATE_THREADS",
  "READ_HISTORY_BEFORE_JOIN",
  "ATTACH_FILES",
  "ADD_REACTIONS",
  "USE_CUSTOM_EMOJI",
  "MENTION_EVERYONE",
  "MANAGE_MESSAGES",
  "CREATE_INVITES",
  "CONNECT_VOICE",
  "SPEAK",
  "VIDEO",
  "SCREEN_SHARE",
  "MUTE_MEMBERS",
  "DEAFEN_MEMBERS",
  "MOVE_MEMBERS",
];

/** What an override says about one permission. */
export type OverrideState = "allow" | "neutral" | "deny";

/** Each permission's state in an override, or all neutral for none. */
export function statesOf(override: ChannelOverride | undefined): Map<PermissionName, OverrideState> {
  const states = new Map<PermissionName, OverrideState>();
  for (const permission of channelLevelPermissions) {
    const allowed = override?.allow.includes(permission) ?? false;
    const denied = override?.deny.includes(permission) ?? false;
    states.set(permission, allowed ? "allow" : denied ? "deny" : "neutral");
  }
  return states;
}

/** The override to save for these states. */
export function specOf(states: ReadonlyMap<PermissionName, OverrideState>): OverrideSpec {
  const allow: PermissionName[] = [];
  const deny: PermissionName[] = [];
  for (const [permission, state] of states) {
    if (state === "allow") {
      allow.push(permission);
    } else if (state === "deny") {
      deny.push(permission);
    }
  }
  return { allow, deny };
}

/** Who an override is for, by name: a role's name or a member's display name. */
export function targetName(
  details: ServerDetails,
  members: readonly ServerMember[],
  target: { targetKind: OverrideTargetKind; targetId: string },
): string {
  if (target.targetKind === "role") {
    return details.roles.find((role) => role.id === target.targetId)?.name ?? target.targetId;
  }
  return members.find((member) => member.user.id === target.targetId)?.user.displayName ?? target.targetId;
}
