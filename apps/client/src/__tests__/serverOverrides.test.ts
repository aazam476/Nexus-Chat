import { channelLevelPermissions, specOf, statesOf, targetName } from "../servers/overrides";
import { bookClub, bookClubMembers } from "../testing/servers";

describe("channel overrides", () => {
  it("read and write each permission's state", () => {
    const states = statesOf({
      targetKind: "role",
      targetId: "r",
      allow: ["SEND_MESSAGES"],
      deny: ["VIEW_CHANNEL"],
    });
    expect(states.get("SEND_MESSAGES")).toBe("allow");
    expect(states.get("VIEW_CHANNEL")).toBe("deny");
    expect(states.get("SPEAK")).toBe("neutral");
    expect(specOf(states)).toEqual({ allow: ["SEND_MESSAGES"], deny: ["VIEW_CHANNEL"] });
    expect(specOf(statesOf(undefined))).toEqual({ allow: [], deny: [] });
    expect(statesOf(undefined).size).toBe(channelLevelPermissions.length);
  });

  it("name who they are for", () => {
    const details = bookClub();
    expect(targetName(details, bookClubMembers, { targetKind: "role", targetId: "role-everyone" })).toBe(
      "@everyone",
    );
    expect(
      targetName(details, bookClubMembers, {
        targetKind: "member",
        targetId: bookClubMembers[1]?.user.id ?? "",
      }),
    ).toBe("Grace");
    expect(targetName(details, bookClubMembers, { targetKind: "role", targetId: "gone" })).toBe("gone");
    expect(targetName(details, bookClubMembers, { targetKind: "member", targetId: "gone" })).toBe("gone");
  });
});
