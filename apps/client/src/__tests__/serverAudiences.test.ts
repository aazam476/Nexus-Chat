import type { ServerChannel, ServerDetails, ServerMember } from "@nexus-chat/api-client";

import { audiencesOf } from "../servers/audiences";

const channel = (
  id: string,
  kind: ServerChannel["kind"],
  audienceId?: string,
  overrides: ServerChannel["overrides"] = [],
): ServerChannel => ({
  id,
  kind,
  name: id,
  topic: "",
  position: 0,
  overrides,
  permissions: ["VIEW_CHANNEL"],
  ...(audienceId === undefined ? {} : { audienceId }),
});

const details: ServerDetails = {
  server: {
    id: "s1",
    name: "Club",
    description: "",
    ownerId: "owner",
    compressMedia: false,
    createdAt: "2026-11-01T00:00:00Z",
  },
  roles: [
    {
      id: "everyone",
      name: "@everyone",
      permissions: ["VIEW_CHANNEL", "READ_HISTORY_BEFORE_JOIN"],
      position: 0,
      everyone: true,
    },
  ],
  channels: [
    channel("category", "category"),
    channel("general", "text", "a1"),
    channel("books", "text", "a1"),
    channel("staff", "text", "a2", [
      { targetKind: "role", targetId: "everyone", allow: [], deny: ["READ_HISTORY_BEFORE_JOIN"] },
    ]),
    channel("lounge", "voice"),
  ],
  myRoleIds: [],
  permissions: ["VIEW_CHANNEL"],
};
const members: ServerMember[] = [
  { user: { id: "sam", username: "sam", displayName: "Sam" }, joinedAt: "2026-11-02T00:00:00Z", roleIds: [] },
];

describe("a server's audiences", () => {
  it("group the text channels by audience and decide who may read their history", () => {
    const audiences = audiencesOf(details, members);
    expect(audiences.map(({ audienceId, channelIds }) => ({ audienceId, channelIds }))).toEqual([
      { audienceId: "a1", channelIds: ["general", "books"] },
      { audienceId: "a2", channelIds: ["staff"] },
    ]);
    const [open, staff] = audiences;
    expect(open?.mayReadHistory("sam", "general")).toBe(true);
    expect(staff?.mayReadHistory("sam", "staff")).toBe(false);
    expect(open?.mayReadHistory("stranger", "general")).toBe(false);
    expect(open?.mayReadHistory("sam", "elsewhere")).toBe(false);
  });
});
