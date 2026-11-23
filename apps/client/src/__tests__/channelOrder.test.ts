import type { ServerChannel } from "@nexus-chat/api-client";

import { moved, movedInto } from "../servers/channelOrder";

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

const channels = [
  channel("loose", "text", 0),
  channel("text", "category", 0),
  channel("voice", "category", 1),
  channel("general", "text", 0, "text"),
  channel("books", "text", 1, "text"),
  channel("lounge", "voice", 0, "voice"),
];

describe("rearranging channels", () => {
  it("moves a channel among its siblings, but not past the ends", () => {
    expect(moved(channels, "books", -1)).toEqual([
      { id: "loose" },
      { id: "text" },
      { id: "books", parentId: "text" },
      { id: "general", parentId: "text" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
    ]);
    expect(moved(channels, "general", -1)).toBeUndefined();
    expect(moved(channels, "lounge", 1)).toBeUndefined();
    expect(moved(channels, "unknown", 1)).toBeUndefined();
  });

  it("moves categories with their channels", () => {
    expect(moved(channels, "voice", -1)).toEqual([
      { id: "loose" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
      { id: "text" },
      { id: "general", parentId: "text" },
      { id: "books", parentId: "text" },
    ]);
    expect(moved(channels, "text", -1)).toBeUndefined();
    expect(moved(channels, "voice", 1)).toBeUndefined();
  });

  it("moves a channel to the end of another category, or out of any", () => {
    expect(movedInto(channels, "general", "voice")).toEqual([
      { id: "loose" },
      { id: "text" },
      { id: "books", parentId: "text" },
      { id: "voice" },
      { id: "lounge", parentId: "voice" },
      { id: "general", parentId: "voice" },
    ]);
    expect(movedInto(channels, "lounge", undefined)).toEqual([
      { id: "loose" },
      { id: "lounge" },
      { id: "text" },
      { id: "general", parentId: "text" },
      { id: "books", parentId: "text" },
      { id: "voice" },
    ]);
    // Categories stay at the top level, and unknown channels change nothing.
    expect(movedInto(channels, "text", "voice")).toHaveLength(6);
    expect(movedInto(channels, "unknown", "voice")).toEqual(movedInto(channels, "text", undefined));
  });
});
