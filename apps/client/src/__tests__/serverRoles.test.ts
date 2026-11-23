import type { ServerDetails, ServerRole } from "@nexus-chat/api-client";

import { grantable, mayManageRole, topPosition } from "../servers/hierarchy";
import { permissionNames } from "../servers/permissions";
import { movedRole } from "../servers/roleOrder";
import { bookClub } from "../testing/servers";

const role = (id: string, position: number, everyone = false): ServerRole => ({
  id,
  name: id,
  permissions: [],
  position,
  everyone,
});

/** Book club with Admins (3), Mods (2) and Helpers (1); Sam has Mods and MANAGE_ROLES. */
function withRoles(permissions: ServerDetails["permissions"] = ["MANAGE_ROLES"]): ServerDetails {
  const details = bookClub();
  return {
    ...details,
    roles: [role("everyone", 0, true), role("helpers", 1), role("mods", 2), role("admins", 3)],
    myRoleIds: ["mods"],
    permissions,
  };
}

describe("the role order", () => {
  it("lets the owner outrank everyone, and others only roles below their own", () => {
    const details = withRoles();
    expect(topPosition(details, details.server.ownerId)).toBe(Number.POSITIVE_INFINITY);
    expect(topPosition(details, "sam")).toBe(2);
    expect(topPosition({ ...details, myRoleIds: [] }, "sam")).toBe(0);
    expect(mayManageRole(details, "sam", role("helpers", 1))).toBe(true);
    expect(mayManageRole(details, "sam", role("mods", 2))).toBe(false);
    expect(mayManageRole(withRoles([]), "sam", role("helpers", 1))).toBe(false);
  });

  it("offers only the permissions the user has, or all to administrators", () => {
    expect(grantable(withRoles(["MANAGE_ROLES", "KICK_MEMBERS"]))).toEqual(["MANAGE_ROLES", "KICK_MEMBERS"]);
    expect(grantable(withRoles(["ADMINISTRATOR"]))).toEqual(permissionNames);
  });

  it("moves roles past their neighbors, below the user's own", () => {
    const details = withRoles();
    const owner = details.server.ownerId;
    expect(movedRole(details, owner, "helpers", -1)).toEqual(["admins", "helpers", "mods"]);
    expect(movedRole(details, owner, "admins", -1)).toBeUndefined();
    expect(movedRole(details, owner, "helpers", 1)).toBeUndefined();
    // Sam may not move Helpers above his own Mods.
    expect(movedRole(details, "sam", "helpers", -1)).toBeUndefined();
    expect(movedRole(details, owner, "unknown", 1)).toBeUndefined();
  });
});
