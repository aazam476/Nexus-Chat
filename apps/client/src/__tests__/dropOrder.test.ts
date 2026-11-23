import type { ServerChannel, ServerDetails, ServerRole } from "@nexus-chat/api-client";

import {
  channelRows,
  dropIndex,
  droppedChannel,
  droppedRole,
  placedChannels,
  placedRoles,
} from "../servers/dropOrder";

const channel = (
  id: string,
  kind: ServerChannel["kind"],
  position: number,
  parentId?: string,
): ServerChannel => ({
  id,
  kind,
  name: id,
  topic: "",
  position,
  overrides: [],
  permissions: [],
  ...(parentId === undefined ? {} : { parentId }),
});

// Shown as: loose, text, general, books, voice, lounge.
const channels = [
  channel("loose", "text", 0),
  channel("text", "category", 0),
  channel("voice", "category", 1),
  channel("general", "text", 0, "text"),
  channel("books", "text", 1, "text"),
  channel("lounge", "voice", 0, "voice"),
];

const rowsOf = (list: readonly ServerChannel[]) => channelRows(list).map((row) => row.id);

describe("where a dragged row lands", () => {
  const layouts = [0, 50, 100, 150].map((y) => ({ y, height: 40 }));

  it("counts the rows whose middle the dragged row's middle passed", () => {
    expect(dropIndex(layouts, 0, 0)).toBe(0);
    expect(dropIndex(layouts, 0, 60)).toBe(1);
    expect(dropIndex(layouts, 0, 400)).toBe(3);
    expect(dropIndex(layouts, 3, -110)).toBe(1);
    expect(dropIndex(layouts, 2, -400)).toBe(0);
  });

  it("leaves a row it knows nothing about where it was", () => {
    expect(dropIndex(layouts, 7, 100)).toBe(7);
  });
});

describe("dropping channels", () => {
  it("lists the channels as the screen shows them", () => {
    expect(rowsOf(channels)).toEqual(["loose", "text", "general", "books", "voice", "lounge"]);
  });

  it("moves a channel within its category", () => {
    // Among the others (loose, text, general, voice, lounge), before general.
    expect(droppedChannel(channels, "books", 2)).toEqual([
      { id: "loose" },
      { id: "text" },
      { id: "books", parentId: "text" },
      { id: "general", parentId: "text" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
    ]);
  });

  it("moves a channel into the category above where it is dropped, or out of any", () => {
    expect(droppedChannel(channels, "general", 5)).toEqual([
      { id: "loose" },
      { id: "text" },
      { id: "books", parentId: "text" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
      { id: "general", parentId: "voice" },
    ]);
    expect(droppedChannel(channels, "lounge", 0)).toEqual([
      { id: "lounge" },
      { id: "loose" },
      { id: "text" },
      { id: "general", parentId: "text" },
      { id: "books", parentId: "text" },
      { id: "voice" },
    ]);
  });

  it("moves a category with its channels", () => {
    // Among the others (loose, text, general, books, lounge), before text.
    expect(droppedChannel(channels, "voice", 1)).toEqual([
      { id: "loose" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
      { id: "text" },
      { id: "general", parentId: "text" },
      { id: "books", parentId: "text" },
    ]);
    // Dropped among its own category's channels, it stays.
    expect(droppedChannel(channels, "text", 3)).toBeUndefined();
  });

  it("changes nothing when a row is dropped where it was, or is unknown", () => {
    expect(droppedChannel(channels, "books", 3)).toBeUndefined();
    expect(droppedChannel(channels, "unknown", 0)).toBeUndefined();
  });

  it("shows a dropped order before the server confirms it", () => {
    const placements = droppedChannel(channels, "general", 5) ?? [];
    const placed = placedChannels([...channels, channel("hidden", "text", 9)], placements);
    expect(rowsOf(placed)).toEqual(["loose", "hidden", "text", "books", "voice", "lounge", "general"]);
    expect(placed.find((row) => row.id === "general")?.parentId).toBe("voice");
    expect(placed.find((row) => row.id === "loose")?.parentId).toBeUndefined();
  });
});

describe("dropping roles", () => {
  const role = (id: string, position: number): ServerRole => ({
    id,
    name: id,
    position,
    permissions: [],
    everyone: position === 0,
  });
  const details = {
    server: { id: "s", ownerId: "owner" },
    roles: [role("everyone", 0), role("member", 1), role("helper", 2), role("mod", 3), role("admin", 4)],
    myRoleIds: ["mod"],
    permissions: ["MANAGE_ROLES"],
  } as unknown as ServerDetails;

  it("moves a role below the user's own", () => {
    // Among the others (admin, mod, helper), last.
    expect(droppedRole(details, "me", "member", 1)).toBeUndefined();
    expect(droppedRole(details, "me", "member", 2)).toEqual(["admin", "mod", "member", "helper"]);
    expect(droppedRole(details, "owner", "admin", 3)).toEqual(["mod", "helper", "member", "admin"]);
  });

  it("never moves a role past one the user may not manage", () => {
    expect(droppedRole(details, "me", "member", 0)).toBeUndefined();
    expect(droppedRole(details, "me", "mod", 3)).toBeUndefined();
    expect(droppedRole(details, "me", "everyone", 0)).toBeUndefined();
  });

  it("changes nothing when a role is dropped where it was", () => {
    expect(droppedRole(details, "me", "helper", 2)).toBeUndefined();
  });

  it("shows a dropped order before the server confirms it, @everyone staying lowest", () => {
    const placed = placedRoles(details, ["admin", "mod", "member", "helper"]);
    const positions = Object.fromEntries(placed.roles.map((r) => [r.id, r.position]));
    expect(positions).toEqual({ everyone: 0, member: 2, helper: 1, mod: 3, admin: 4 });
  });
});
