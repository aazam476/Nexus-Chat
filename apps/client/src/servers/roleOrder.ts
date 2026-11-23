/**
 * Rearranging roles: moving one up or down past its neighbor. The server
 * takes every role but `@everyone`, highest first, and only lets the user
 * move roles below their own highest, so a move is offered only when both
 * roles it swaps are below it.
 */
import type { ServerDetails } from "@nexus-chat/api-client";

import { mayManageRole, rolesHighestFirst } from "./hierarchy";

/** The role IDs, highest first, after moving `roleId` up (-1) or down (1); undefined when it cannot move so. */
export function movedRole(
  details: ServerDetails,
  userId: string,
  roleId: string,
  by: -1 | 1,
): string[] | undefined {
  const order = rolesHighestFirst(details).filter((role) => !role.everyone);
  const at = order.findIndex((role) => role.id === roleId);
  const role = order[at];
  const other = order[at + by];
  if (
    role === undefined ||
    other === undefined ||
    !mayManageRole(details, userId, role) ||
    !mayManageRole(details, userId, other)
  ) {
    return undefined;
  }
  const ids = order.map((candidate) => candidate.id);
  ids[at] = other.id;
  ids[at + by] = role.id;
  return ids;
}
