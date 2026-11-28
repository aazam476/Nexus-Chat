import { decodeEvidence, encodeEvidence } from "@nexus-chat/crypto";

import { utf8 } from "../messaging/engine/bytes";
import type { ReportPlace, ReportToCheck } from "../messaging/engine/reports";
import type { Messenger } from "../messaging/messenger";
import { audiencesOf } from "../servers/audiences";
import { signingInstance, signReceipt } from "../testing/directives";
import { FakeInstance } from "../testing/fakeDelivery";
import { dm, grace as graceUser, messengersInDm } from "../testing/messengers";
import { audienceId, bookClub, general, messengersInServer } from "../testing/servers";

jest.setTimeout(60_000);

const audience = audiencesOf(bookClub(), [])[0];

/** The instance's part: a signed receipt for the stored copy, as the server makes it. */
function receiptFor(
  messageId: string,
  sender: string,
  key: Uint8Array,
  extra: object = {},
): ReturnType<typeof signReceipt> {
  return signReceipt(
    {
      conversationId: audienceId,
      channelId: general,
      messageId,
      senderUsername: sender,
      epoch: 1,
      sentAt: "2026-11-20T12:00:00Z",
      ...extra,
    },
    key,
  );
}

async function inServer(): Promise<{
  key: Uint8Array;
  ada: Messenger;
  grace: Messenger;
  place: ReportPlace;
}> {
  if (audience === undefined) {
    throw new Error("no audience");
  }
  const instance = new FakeInstance();
  const key = await signingInstance(instance);
  const { ada, grace } = await messengersInServer(instance);
  return { key, ada, grace, place: { kind: "channel", audience, channelId: general } };
}

describe("reports", () => {
  it("prove to a moderator who sent what", async () => {
    const { key, ada, grace, place } = await inServer();
    if (place.kind !== "channel") {
      throw new Error("a channel");
    }
    const id = await grace.sendInChannel(place.audience, general, {
      kind: "text",
      text: "you are all fools",
    });
    await ada.sync(audienceId);
    const evidence = await ada.reportEvidence(place, id);
    expect(evidence).toBeDefined();
    const report: ReportToCheck = {
      messageId: id,
      evidence: evidence ?? new Uint8Array(),
      ...receiptFor(id, "grace", key, {
        epoch: Number(decodeEvidence(evidence ?? new Uint8Array())?.epoch),
      }),
    };
    const verdict = await ada.checkReport(report);
    expect(verdict).toMatchObject({
      verified: true,
      sender: "@grace@chat.example.com",
      payload: { kind: "text", text: "you are all fools" },
    });
    expect(await ada.reportEvidence(place, "0195f0e0-0000-7000-8000-00000000ffff")).toBeUndefined();
  });

  it("reject forgeries", async () => {
    const { key, ada, grace, place } = await inServer();
    if (place.kind !== "channel") {
      throw new Error("a channel");
    }
    const theirs = await grace.sendInChannel(place.audience, general, { kind: "text", text: "see you at 8" });
    const mine = await ada.sendInChannel(place.audience, general, {
      kind: "text",
      text: "you are all fools",
    });
    await ada.sync(audienceId);
    const real = decodeEvidence((await ada.reportEvidence(place, theirs)) ?? new Uint8Array());
    const own = (await ada.reportEvidence(place, mine)) ?? new Uint8Array();
    if (real === undefined) {
      throw new Error("no evidence");
    }
    const epoch = { epoch: Number(real.epoch) };
    const check = (
      messageId: string,
      evidence: Uint8Array,
      receipt = receiptFor(messageId, "grace", key, epoch),
    ) => ada.checkReport({ messageId, evidence, ...receipt });

    // Words the sender never wrote, in place of theirs.
    const reworded = utf8(
      new TextDecoder().decode(real.plaintext).replace("see you at 8", "you are all fools"),
    );
    expect(await check(theirs, encodeEvidence({ ...real, plaintext: reworded }))).toEqual({
      verified: false,
      problem: "signature",
    });
    // The reporter's own message, passed off as the sender's.
    expect(await check(mine, own)).toEqual({ verified: false, problem: "sender" });
    // Evidence about one message, with the receipt for another.
    expect(await check(mine, encodeEvidence(real))).toEqual({ verified: false, problem: "mismatch" });
    expect(
      await check(theirs, encodeEvidence(real), receiptFor(theirs, "grace", key, { epoch: 99 })),
    ).toEqual({
      verified: false,
      problem: "mismatch",
    });
    // A receipt the instance never signed, or one changed after signing.
    const forger = await signingInstance(new FakeInstance());
    expect(await check(theirs, encodeEvidence(real), receiptFor(theirs, "grace", forger, epoch))).toEqual({
      verified: false,
      problem: "receipt",
    });
    const changed = receiptFor(theirs, "grace", key, epoch);
    changed.receipt = utf8(new TextDecoder().decode(changed.receipt).replace("grace", "ada"));
    expect(await check(theirs, encodeEvidence(real), changed)).toEqual({
      verified: false,
      problem: "receipt",
    });
    expect(await check(theirs, new Uint8Array([1, 2, 3]))).toEqual({ verified: false, problem: "evidence" });
    // The receipt says another user sent it.
    expect(await check(theirs, encodeEvidence(real), receiptFor(theirs, "ada", key, epoch))).toEqual({
      verified: false,
      problem: "sender",
    });
  });

  it("can be made about a direct message", async () => {
    const { ada, grace } = await messengersInDm();
    const id = await grace.send(dm.id, { kind: "text", text: "hello" });
    await ada.sync(dm.id);
    expect(await ada.reportEvidence({ kind: "conversation", conversationId: dm.id }, id)).toBeDefined();
    expect(graceUser.username).toBe("grace");
  });
});
