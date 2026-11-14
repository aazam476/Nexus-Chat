import type { ChannelOverride, PermissionName, ServerRole } from "@nexus-chat/api-client";

import { channelPermissions, permissionNames, serverPermissions } from "../servers/permissions";

const defaults: PermissionName[] = [
  "VIEW_CHANNEL",
  "SEND_MESSAGES",
  "SEND_IN_THREADS",
  "CREATE_THREADS",
  "READ_HISTORY_BEFORE_JOIN",
  "ATTACH_FILES",
  "ADD_REACTIONS",
  "USE_CUSTOM_EMOJI",
  "CREATE_INVITES",
  "CONNECT_VOICE",
  "SPEAK",
  "VIDEO",
  "SCREEN_SHARE",
];
const role = (id: string, permissions: PermissionName[], everyone = false): ServerRole => ({
  id,
  name: id,
  permissions,
  position: everyone ? 0 : 1,
  everyone,
});
const server = {
  ownerId: "owner",
  roles: [
    role("everyone", defaults, true),
    role("moderators", ["KICK_MEMBERS", "TIMEOUT_MEMBERS", "MANAGE_MESSAGES"]),
    role("admins", ["ADMINISTRATOR"]),
  ],
};
const dana = { userId: "dana", roleIds: ["moderators"] };
const sam = { userId: "sam", roleIds: [] };
const override = (
  targetKind: "role" | "member",
  targetId: string,
  deny: PermissionName[],
  allow: PermissionName[] = [],
): ChannelOverride => ({ targetKind, targetId, allow, deny });
const announcements = [
  override("role", "everyone", ["SEND_MESSAGES", "ADD_REACTIONS"]),
  override("role", "moderators", [], ["SEND_MESSAGES"]),
];

describe("server permissions", () => {
  it("follow the worked example in docs/permissions.md", () => {
    expect(serverPermissions(server, dana).has("KICK_MEMBERS")).toBe(true);
    expect([...serverPermissions(server, sam)].sort()).toEqual([...defaults].sort());

    const danaHere = channelPermissions(server, dana, announcements);
    expect(danaHere.has("SEND_MESSAGES")).toBe(true);
    expect(danaHere.has("MANAGE_MESSAGES")).toBe(true);
    expect(danaHere.has("ADD_REACTIONS")).toBe(false);
    const samHere = channelPermissions(server, sam, announcements);
    expect(samHere.has("VIEW_CHANNEL")).toBe(true);
    expect(samHere.has("SEND_MESSAGES")).toBe(false);
    expect(samHere.has("ADD_REACTIONS")).toBe(false);
  });

  it("let a member's own override have the last word, and hide a channel without VIEW_CHANNEL", () => {
    const hidden = [
      override("role", "everyone", ["VIEW_CHANNEL"]),
      override("role", "moderators", [], ["VIEW_CHANNEL"]),
    ];
    expect(channelPermissions(server, sam, hidden).size).toBe(0);
    expect(channelPermissions(server, dana, hidden).has("VIEW_CHANNEL")).toBe(true);
    const danaDenied = [...hidden, override("member", "dana", ["VIEW_CHANNEL"])];
    expect(channelPermissions(server, dana, danaDenied).size).toBe(0);
    const samAllowed = [...hidden, override("member", "sam", [], ["VIEW_CHANNEL"])];
    expect(channelPermissions(server, sam, samAllowed).has("VIEW_CHANNEL")).toBe(true);
  });

  it("let a role's allow beat another role's deny", () => {
    const twoRoles = { userId: "lee", roleIds: ["moderators", "quiet"] };
    const withQuiet = { ...server, roles: [...server.roles, role("quiet", [])] };
    const overrides = [
      override("role", "quiet", ["SEND_MESSAGES"]),
      override("role", "moderators", [], ["SEND_MESSAGES"]),
    ];
    expect(channelPermissions(withQuiet, twoRoles, overrides).has("SEND_MESSAGES")).toBe(true);
  });

  it("give the owner and administrators everything, overrides or not", () => {
    const blocked = [
      override("role", "everyone", ["VIEW_CHANNEL"]),
      override("member", "owner", ["VIEW_CHANNEL"]),
    ];
    expect(channelPermissions(server, { userId: "owner", roleIds: [] }, blocked).size).toBe(
      permissionNames.length,
    );
    const admin = { userId: "ada", roleIds: ["admins"], timedOut: true };
    expect(serverPermissions(server, admin).size).toBe(permissionNames.length);
    expect(channelPermissions(server, admin, blocked).size).toBe(permissionNames.length);
  });

  it("leave a timed-out member only reading", () => {
    const quiet = { ...sam, timedOut: true };
    expect([...serverPermissions(server, quiet)].sort()).toEqual([
      "READ_HISTORY_BEFORE_JOIN",
      "VIEW_CHANNEL",
    ]);
    expect([...channelPermissions(server, quiet, [])].sort()).toEqual([
      "READ_HISTORY_BEFORE_JOIN",
      "VIEW_CHANNEL",
    ]);
    expect(channelPermissions({ ownerId: "owner", roles: [] }, quiet, []).size).toBe(0);
  });
});
