/**
 * A server for screen tests: Book club, owned by Ada, with Grace as a
 * member, a category of text channels, a voice channel, and the fake API
 * answers that describe it. `messengersInServer` gives both of them a
 * messenger already in the channels' audience group.
 */
import type { ServerChannel, ServerDetails, ServerMember } from "@nexus-chat/api-client";

import type { Messenger } from "../messaging/messenger";
import { audiencesOf } from "../servers/audiences";
import { FakeInstance } from "./fakeDelivery";
import type { FakeServer } from "./fakeServer";
import { grace, readyMessenger } from "./messengers";
import { ada } from "./renderWithApp";

/** Book club's ID. */
export const serverId = "0195f0e0-0000-7000-8000-0000000005e1";
/** The audience of its text channels. */
export const audienceId = "0195f0e0-0000-7000-8000-0000000000a1";
/** Its #general channel. */
export const general = "0195f0e0-0000-7000-8000-00000000c001";
const all = ["VIEW_CHANNEL", "SEND_MESSAGES", "READ_HISTORY_BEFORE_JOIN", "CREATE_INVITES"] as const;

const channel = (
  id: string,
  kind: ServerChannel["kind"],
  name: string,
  extra: Partial<ServerChannel> = {},
): ServerChannel => ({
  id,
  kind,
  name,
  topic: "",
  position: 0,
  overrides: [],
  permissions: [...all],
  ...extra,
});

/** Book club as a member sees it; `extra` changes the server's permissions or channels. */
export function bookClub(extra: Partial<ServerDetails> = {}): ServerDetails {
  return {
    server: {
      id: serverId,
      name: "Book club",
      description: "We read.",
      ownerId: ada.id,
      compressMedia: false,
      createdAt: "2026-11-01T00:00:00Z",
    },
    roles: [{ id: "role-everyone", name: "@everyone", permissions: [...all], position: 0, everyone: true }],
    channels: [
      channel("cat-text", "category", "Text channels"),
      channel(general, "text", "general", { parentId: "cat-text", topic: "Say hello", audienceId }),
      channel("voice-1", "voice", "Lounge", { position: 1 }),
    ],
    myRoleIds: [],
    permissions: [...all, "MANAGE_SERVER", "MANAGE_CHANNELS", "MANAGE_ROLES"],
    ...extra,
  };
}

/** Ada and Grace, Book club's members. */
export const bookClubMembers: ServerMember[] = [
  {
    user: { id: ada.id, username: ada.username, displayName: ada.displayName },
    joinedAt: "2026-11-01T00:00:00Z",
    roleIds: [],
  },
  { user: grace, joinedAt: "2026-11-02T00:00:00Z", roleIds: [] },
];

/** Makes the fake API describe `details` and its members. */
export function answerServer(server: FakeServer, details: ServerDetails = bookClub()): void {
  server.answer("GET /api/v1/servers", { status: 200, body: [details.server] });
  server.answer(`GET /api/v1/servers/${details.server.id}`, { status: 200, body: details });
  server.answer(`GET /api/v1/servers/${details.server.id}/members`, { status: 200, body: bookClubMembers });
}

/** Ada's and Grace's messengers, both in Book club's audience group. */
export async function messengersInServer(): Promise<{ ada: Messenger; grace: Messenger }> {
  const instance = new FakeInstance();
  instance.addChannel(general, audienceId);
  const adaMessenger = await readyMessenger(instance, ada, "ada-laptop");
  const graceMessenger = await readyMessenger(instance, grace, "grace-phone");
  const [audience] = audiencesOf(bookClub(), bookClubMembers);
  const people = bookClubMembers.map((member) => member.user);
  if (audience !== undefined) {
    await adaMessenger.openAudience(audience, people);
    await graceMessenger.openAudience(audience, people);
    await adaMessenger.sync(audienceId);
  }
  return { ada: adaMessenger, grace: graceMessenger };
}
