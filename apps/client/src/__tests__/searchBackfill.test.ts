import type { Messenger } from "../messaging/messenger";
import { audiencesOf } from "../servers/audiences";
import { FakeInstance } from "../testing/fakeDelivery";
import { dm, grace, readyMessenger } from "../testing/messengers";
import { ada } from "../testing/renderWithApp";
import { bookClub, general, messengersInServer } from "../testing/servers";

jest.setTimeout(60_000);

const yes = () => Promise.resolve(true);
const no = () => Promise.resolve(false);

/** Moves every copy the instance holds for `id` to early 2025, before the recent window. */
function backdate(instance: FakeInstance, ...ids: string[]): void {
  let second = 0;
  const old = () => new Date(Date.UTC(2025, 0, 1, 0, 0, (second += 1))).toISOString();
  for (const id of ids) {
    instance.messages.get(id)?.forEach((message) => (message.createdAt = old()));
    instance.archives.get(id)?.forEach((copy) => (copy.createdAt = old()));
    instance.channelArchives.get(id)?.forEach((copy) => (copy.createdAt = old()));
  }
}

async function texts(messenger: Messenger, query: string): Promise<string[]> {
  return (await messenger.search(query)).map((hit) => hit.text).sort();
}

describe("searching older messages", () => {
  it("indexes a conversation's whole history when asked, and can stop and resume", async () => {
    const instance = new FakeInstance();
    const adaMessenger = await readyMessenger(instance, ada, "ada-laptop");
    const graceMessenger = await readyMessenger(instance, grace, "grace-phone");
    instance.addConversation(dm.id);
    await adaMessenger.open(dm.id, dm.members);
    await graceMessenger.open(dm.id, dm.members);
    for (const text of ["old plan one", "old plan two", "old plan three"]) {
      await graceMessenger.send(dm.id, { kind: "text", text });
    }
    backdate(instance, dm.id);
    await adaMessenger.sync(dm.id);
    expect(await texts(adaMessenger, "plan")).toEqual([]);

    // Not a good time: what the device holds is indexed, the archive waits.
    const waiting = await adaMessenger.searchOlder(dm.id, no);
    expect(waiting).toEqual({ indexed: 3, done: false });
    expect(await texts(adaMessenger, "plan")).toEqual(["old plan one", "old plan three", "old plan two"]);

    const finished = await adaMessenger.searchOlder(dm.id, yes);
    expect(finished.done).toBe(true);
    // From now on, old messages arriving here are indexed too.
    await graceMessenger.send(dm.id, { kind: "text", text: "old plan four" });
    backdate(instance, dm.id);
    await adaMessenger.sync(dm.id);
    expect(await texts(adaMessenger, "four")).toEqual(["old plan four"]);
    expect(await adaMessenger.searchOlder(dm.id, yes)).toMatchObject({ done: true });
  });

  it("reads a new device's missing history from the archive", async () => {
    const instance = new FakeInstance();
    const adaMessenger = await readyMessenger(instance, ada, "ada-laptop");
    instance.addConversation(dm.id);
    await adaMessenger.open(dm.id, dm.members);
    await adaMessenger.send(dm.id, { kind: "text", text: "archived note" });
    backdate(instance, dm.id);
    // Another device of Grace's has only the archive to go on.
    const graceMessenger = await readyMessenger(instance, grace, "grace-phone");
    await graceMessenger.open(dm.id, dm.members);
    const progress = await graceMessenger.searchOlder(dm.id, yes);
    expect(progress.done).toBe(true);
  });

  it("works for server channels too", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInServer();
    const audience = audiencesOf(bookClub(), [])[0];
    if (audience === undefined) {
      throw new Error("no audience");
    }
    await graceMessenger.sendInChannel(audience, general, { kind: "text", text: "chapter notes" });
    await adaMessenger.sync(audience.audienceId);
    expect(await texts(adaMessenger, "chapter")).toEqual(["chapter notes"]);
    expect(await adaMessenger.searchOlder(general, yes)).toMatchObject({ done: true });
  });
});
