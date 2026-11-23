/**
 * The servers half of the fake instance: one server the user owns, built
 * from the dataset, with its channels under two categories, its roles and
 * its members. Its channels have no encryption group yet, so the screens
 * show the server without the app joining any.
 */
import type { Dataset } from "@nexus-chat/fixtures";

import type { Answer } from "./fakeMessaging.js";

/** The server the screens show. */
export const serverId = "0195f0e0-0000-7000-8000-00000000e001";
const createdAt = "2026-11-01T09:00:00Z";
const id = (kind: string, index: number) =>
  `0195f0e0-0000-7000-8000-0000000${kind}${String(index).padStart(4, "0")}`;

/** Every permission, as the owner holds them; members' roles hold the channel ones. */
const permissions = [
  "VIEW_CHANNEL",
  "SEND_MESSAGES",
  "SEND_IN_THREADS",
  "CREATE_THREADS",
  "READ_HISTORY_BEFORE_JOIN",
  "ATTACH_FILES",
  "ADD_REACTIONS",
  "USE_CUSTOM_EMOJI",
  "CONNECT_VOICE",
  "SPEAK",
  "VIDEO",
  "SCREEN_SHARE",
  "CREATE_INVITES",
  "MENTION_EVERYONE",
  "MANAGE_MESSAGES",
  "MUTE_MEMBERS",
  "DEAFEN_MEMBERS",
  "MOVE_MEMBERS",
  "TIMEOUT_MEMBERS",
  "KICK_MEMBERS",
  "BAN_MEMBERS",
  "VIEW_AUDIT_LOG",
  "MANAGE_EMOJI",
  "MANAGE_CHANNELS",
  "MANAGE_ROLES",
  "MANAGE_SERVER",
  "ADMINISTRATOR",
];

/** The server's channels: a text category with the text channels, then a voice category. */
function channels(dataset: Dataset): unknown[] {
  const { categories, textChannels, voiceChannels } = dataset.servers;
  const channel = (kind: string, name: string, position: number, extra: object = {}) => ({
    kind,
    name,
    topic: "",
    position,
    overrides: [],
    permissions,
    ...extra,
  });
  return [
    channel("category", categories[0], 0, { id: id("c", 1) }),
    channel("category", categories[1], 1, { id: id("c", 2) }),
    ...textChannels.map((text, index) =>
      channel("text", text.name, index, { id: id("a", index), topic: text.topic, parentId: id("c", 1) }),
    ),
    ...voiceChannels.map((name, index) =>
      channel("voice", name, index, { id: id("b", index), parentId: id("c", 2) }),
    ),
  ];
}

/** The server's roles, highest first in the dataset, above `@everyone`. */
function roles(dataset: Dataset): unknown[] {
  const { roles: named } = dataset.servers;
  return [
    { id: id("d", 0), name: "@everyone", permissions: permissions.slice(0, 13), position: 0, everyone: true },
    ...named.map((role, index) => ({
      id: id("d", index + 1),
      name: role.name,
      permissions: permissions.slice(0, 15),
      position: named.length - index,
      everyone: false,
      ...(role.color === undefined ? {} : { color: role.color }),
    })),
  ];
}

/** The server as the list and its own screens show it. */
function server(dataset: Dataset): unknown {
  const { name, description } = dataset.servers;
  return { id: serverId, name, description, ownerId: dataset.user.id, compressMedia: false, createdAt };
}

/** The user with the highest role, and each contact with the lowest. */
function members(dataset: Dataset): unknown[] {
  const { user, messaging, servers } = dataset;
  const lowest = id("d", servers.roles.length);
  return [
    {
      user: { id: user.id, username: user.username, displayName: user.displayName },
      joinedAt: createdAt,
      roleIds: [id("d", 1)],
    },
    ...messaging.contacts.map((contact) => ({ user: contact, joinedAt: createdAt, roleIds: [lowest] })),
  ];
}

/** The answer to `method path`, or undefined when it is not about servers. */
export function answerServers(dataset: Dataset, method: string, path: string): Answer | undefined {
  switch (`${method} ${path}`) {
    case "GET /api/v1/servers":
      return [200, [server(dataset)]];
    case `GET /api/v1/servers/${serverId}`:
      return [
        200,
        {
          server: server(dataset),
          roles: roles(dataset),
          channels: channels(dataset),
          myRoleIds: [id("d", 1)],
          permissions,
        },
      ];
    case `GET /api/v1/servers/${serverId}/members`:
      return [200, members(dataset)];
    case `GET /api/v1/servers/${serverId}/read-state`:
      return [200, [{ channelId: id("a", 0), lastReadId: 0, unreadCount: dataset.servers.channelUnread }]];
    case `GET /api/v1/servers/${serverId}/invites`:
      return [200, []];
  }
  return undefined;
}
