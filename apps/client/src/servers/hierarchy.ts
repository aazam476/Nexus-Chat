/**
 * The role order as the server enforces it (docs/permissions.md): someone
 * manages only roles below their own highest, the owner every role, and
 * nobody grants a permission they do not have unless they are an
 * administrator. The screens use this to offer only what will work.
 */
import type { PermissionName, ServerDetails, ServerRole } from "@nexus-chat/api-client";

import { permissionNames } from "./permissions";

/** The position of the user's highest role; the owner outranks every role. */
export function topPosition(details: ServerDetails, userId: string): number {
  if (details.server.ownerId === userId) {
    return Number.POSITIVE_INFINITY;
  }
  return details.roles
    .filter((role) => details.myRoleIds.includes(role.id))
    .reduce((top, role) => Math.max(top, role.position), 0);
}

/** Whether the user may change, give or take `role`. */
export function mayManageRole(details: ServerDetails, userId: string, role: ServerRole): boolean {
  return details.permissions.includes("MANAGE_ROLES") && role.position < topPosition(details, userId);
}

/** The permissions the user may allow or deny for others. */
export function grantable(details: ServerDetails): readonly PermissionName[] {
  return details.permissions.includes("ADMINISTRATOR") ? permissionNames : details.permissions;
}

/** The roles, highest first, as role lists show them. */
export function rolesHighestFirst(details: ServerDetails): ServerRole[] {
  return [...details.roles].sort((a, b) => b.position - a.position);
}
