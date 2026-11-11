/**
 * Ready messengers for screen tests: Ada (the signed-in user of
 * renderWithApp) and Grace, on one in-memory instance, sharing a DM.
 */
import type { Conversation } from "@nexus-chat/api-client";

import { startMessenger, type Messenger } from "../messaging/messenger";
import { createMemoryDatabase } from "../storage/localDatabase";
import { FakeInstance } from "./fakeDelivery";
import { ada } from "./renderWithApp";

/** Ada's friend Grace, as the server shows her. */
export const grace = { id: "0195f0e0-0000-7000-8000-000000000002", username: "grace", displayName: "Grace" };

/** The DM between Ada and Grace, as the server lists it. */
export const dm: Conversation = {
  id: "dm",
  kind: "dm",
  mlsGroupId: "AAAAAAAAAAAAAAAAAAAAAA==",
  createdAt: "2026-11-10T12:00:00Z",
  members: [{ id: ada.id, username: ada.username, displayName: ada.displayName }, grace],
};

async function ready(
  instance: FakeInstance,
  user: { id: string; username: string },
  deviceId: string,
): Promise<Messenger> {
  const start = await startMessenger({
    api: instance.api(user.id, deviceId),
    database: createMemoryDatabase(),
    user,
    where: { domain: "chat.example.com", deviceId },
    instanceKey: new Uint8Array(32),
  });
  if (start.status !== "ready") {
    throw new Error("a new account's messenger is ready at once");
  }
  return start.messenger;
}

/** Ada's and Grace's messengers, both in the DM. */
export async function messengersInDm(): Promise<{ ada: Messenger; grace: Messenger }> {
  const instance = new FakeInstance();
  const adaMessenger = await ready(instance, ada, "ada-laptop");
  const graceMessenger = await ready(instance, grace, "grace-phone");
  instance.addConversation(dm.id);
  await adaMessenger.open(dm.id, dm.members);
  await graceMessenger.open(dm.id, dm.members);
  return { ada: adaMessenger, grace: graceMessenger };
}
