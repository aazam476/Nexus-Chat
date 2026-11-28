import type { Report as ApiReport } from "@nexus-chat/api-client";
import { decodeEvidence } from "@nexus-chat/crypto";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";

import Report from "../../app/report";
import ServerReports from "../../app/server/[id]/reports";
import { toBase64 } from "../messaging/engine/bytes";
import type { Messenger } from "../messaging/messenger";
import { audiencesOf } from "../servers/audiences";
import { signingInstance, signReceipt } from "../testing/directives";
import { FakeInstance } from "../testing/fakeDelivery";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { dm, messengersInDm, renderAs } from "../testing/messengers";
import {
  answerServer,
  audienceId,
  bookClub,
  general,
  messengersInServer,
  serverId,
} from "../testing/servers";

jest.setTimeout(60_000);

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};
const audience = audiencesOf(bookClub(), [])[0];

/** Grace's message in #general, which Ada's device holds; signed with a key the instance holds. */
async function channelMessage(): Promise<{ ada: Messenger; key: Uint8Array; messageId: string }> {
  if (audience === undefined) {
    throw new Error("no audience");
  }
  const instance = new FakeInstance();
  const key = await signingInstance(instance);
  const { ada, grace } = await messengersInServer(instance);
  const messageId = await grace.sendInChannel(audience, general, { kind: "text", text: "you are all fools" });
  await ada.sync(audienceId);
  return { ada, key, messageId };
}

function serverWithReports(answers: (server: FakeServer) => void): FakeServer {
  const server = fakeServer();
  answerServer(server);
  answers(server);
  return server;
}

describe("reporting a message", () => {
  it("sends the message's evidence and the reporter's comment", async () => {
    const { ada, messageId } = await channelMessage();
    params({ serverId, channelId: general, messageId });
    const server = serverWithReports((fake) => {
      fake.answer("POST /api/v1/reports", { status: 201, body: { id: "report-1" } });
    });
    await renderAs(<Report />, ada, server);
    expect(screen.getByText(/^The server's moderators will see this message/)).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("report-comment"), "  rude  ");
    await fireEvent.press(screen.getByTestId("report-send"));
    expect(await screen.findByText("Report sent. Thank you.")).toBeOnTheScreen();
    const body = server.requests.find((request) => request.path === "/api/v1/reports")?.body as Record<
      string,
      string
    >;
    expect(body).toMatchObject({ serverId, channelId: general, messageId, comment: "rude" });
    expect(body.evidence?.length).toBeGreaterThan(100);
  });

  it("says why a report was not sent", async () => {
    const { ada, messageId } = await channelMessage();
    params({ serverId, channelId: general, messageId });
    const server = serverWithReports((fake) => {
      fake.answer("POST /api/v1/reports", {
        status: 409,
        body: { code: "already_reported", message: "again" },
      });
    });
    await renderAs(<Report />, ada, server);
    await fireEvent.press(screen.getByTestId("report-send"));
    expect(await screen.findByText("You already reported this message.")).toBeOnTheScreen();
  });

  it("cannot prove a message this device cannot open", async () => {
    const { ada } = await channelMessage();
    params({ serverId, channelId: general, messageId: "0195f0e0-0000-7000-8000-00000000ffff" });
    await renderAs(
      <Report />,
      ada,
      serverWithReports(() => undefined),
    );
    await fireEvent.press(screen.getByTestId("report-send"));
    expect(await screen.findByText(/^This device can't open this message's stored copy/)).toBeOnTheScreen();
  });

  it("works for direct messages too, for the instance's admins", async () => {
    const { ada, grace } = await messengersInDm();
    const messageId = await grace.send(dm.id, { kind: "text", text: "hello" });
    await ada.sync(dm.id);
    params({ conversationId: dm.id, messageId });
    const server = serverWithReports((fake) => {
      fake.answer("POST /api/v1/reports", { status: 500, body: {} });
    });
    await renderAs(<Report />, ada, server);
    expect(screen.getByText(/^The instance's admins will see this message/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("report-send"));
    expect(await screen.findByText("The report could not be sent. Try again.")).toBeOnTheScreen();
  });
});

describe("a server's reports", () => {
  it("are checked on the moderator's device, and resolved or passed on", async () => {
    const { ada, key, messageId } = await channelMessage();
    if (audience === undefined) {
      throw new Error("no audience");
    }
    const evidence =
      (await ada.reportEvidence({ kind: "channel", audience, channelId: general }, messageId)) ??
      new Uint8Array();
    const epoch = Number(decodeEvidence(evidence)?.epoch);
    const receipt = signReceipt(
      {
        conversationId: audienceId,
        channelId: general,
        messageId,
        senderUsername: "grace",
        epoch,
        sentAt: "2026-11-20T12:00:00Z",
      },
      key,
    );
    const report = (id: string, extra: Partial<ApiReport> = {}): ApiReport => ({
      id,
      messageId,
      evidence: toBase64(evidence),
      receipt: toBase64(receipt.receipt),
      receiptSignature: toBase64(receipt.receiptSignature),
      comment: "rude",
      status: "open",
      escalated: false,
      resolution: "",
      createdAt: "2026-11-20T12:05:00Z",
      ...extra,
    });
    params({ id: serverId });
    const base = `/api/v1/servers/${serverId}/reports`;
    const server = serverWithReports((fake) => {
      fake.answer(`GET ${base}`, {
        status: 200,
        body: [report("real"), report("forged", { receiptSignature: toBase64(new Uint8Array(64)) })],
      });
      fake.answer(`POST ${base}/real/resolve`, { status: 204 });
      fake.answer(`POST ${base}/real/escalate`, { status: 204 });
    });
    await renderAs(<ServerReports />, ada, server);
    expect(await screen.findByText(/^Verified: sent by ⁨@grace@chat.example.com⁩/)).toBeOnTheScreen();
    expect(screen.getByText("you are all fools")).toBeOnTheScreen();
    expect(screen.getByText(/^This report could not be verified: the server's receipt/)).toBeOnTheScreen();
    expect(screen.getAllByText("The reporter said: ⁨rude⁩")).toHaveLength(2);

    await fireEvent.press(screen.getByTestId("report-escalate-real"));
    await fireEvent.changeText(screen.getByTestId("report-note-real"), "Warned them");
    await fireEvent.press(screen.getByTestId("report-resolve-real"));
    await waitFor(() => {
      expect(server.requests.find((request) => request.path === `${base}/real/resolve`)?.body).toEqual({
        resolution: "Warned them",
      });
    });
    expect(server.requests.some((request) => request.path === `${base}/real/escalate`)).toBe(true);

    server.answer(`GET ${base}`, { status: 200, body: [] });
    await fireEvent.press(screen.getByTestId("reports-status-1"));
    expect(await screen.findByText("No reports here.")).toBeOnTheScreen();
  });
});
