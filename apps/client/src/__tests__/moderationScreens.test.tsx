import type { AuditEntry } from "@nexus-chat/api-client";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import AuditLog from "../../app/server/[id]/audit-log";
import Bans from "../../app/server/[id]/bans";
import Member from "../../app/server/[id]/member/[userId]";
import ServerSettings from "../../app/server/[id]/settings";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { ada } from "../testing/renderWithApp";
import { answerServer, bookClub, bookClubMembers, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);

const base = `/api/v1/servers/${serverId}`;
const grace = bookClubMembers[1]?.user.id ?? "";
const moderator = [
  "KICK_MEMBERS",
  "BAN_MEMBERS",
  "TIMEOUT_MEMBERS",
  "VIEW_AUDIT_LOG",
  "MANAGE_MESSAGES",
] as const;

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};
const requestsTo = (server: FakeServer, method: string, path: string) =>
  server.requests.filter((request) => request.method === method && request.path === path);

async function render(
  ui: React.ReactElement,
  permissions: readonly string[] = moderator,
  prepare: (server: FakeServer) => void = () => undefined,
): Promise<FakeServer> {
  const { ada: messenger } = await messengersInServer();
  const server = fakeServer();
  const details = bookClub();
  answerServer(server, {
    ...details,
    permissions: [...details.permissions, ...permissions] as typeof details.permissions,
  });
  prepare(server);
  await renderAs(ui, messenger, server);
  return server;
}

describe("moderating a member", () => {
  beforeEach(() => {
    params({ id: serverId, userId: grace });
  });

  it("times them out, kicks and bans them", async () => {
    const server = await render(<Member />);
    server.answer(`PUT ${base}/members/${grace}/timeout`, { status: 204 });
    server.answer(`POST ${base}/members/${grace}/kick`, { status: 204 });
    server.answer(`PUT ${base}/bans/${grace}`, { status: 204 });
    expect(await screen.findByText("Moderation")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("member-timeout-length-2"));
    await fireEvent.press(screen.getByTestId("member-timeout"));
    await waitFor(() => {
      expect(requestsTo(server, "PUT", `${base}/members/${grace}/timeout`)[0]?.body).toEqual({
        seconds: 86_400,
      });
    });

    await fireEvent.changeText(screen.getByTestId("member-reason"), "  spam  ");
    await fireEvent.press(screen.getByTestId("member-kick"));
    await fireEvent.press(screen.getByTestId("member-kick-confirm"));
    await waitFor(() => {
      expect(requestsTo(server, "POST", `${base}/members/${grace}/kick`)[0]?.body).toEqual({
        reason: "spam",
      });
    });
    expect(router.back).toHaveBeenCalled();

    await fireEvent.press(screen.getByTestId("member-purge-3"));
    await fireEvent.press(screen.getByTestId("member-ban"));
    await fireEvent.press(screen.getByTestId("member-ban-confirm"));
    await waitFor(() => {
      expect(requestsTo(server, "PUT", `${base}/bans/${grace}`)[0]?.body).toEqual({
        reason: "spam",
        deleteMessageSeconds: 604_800,
      });
    });
  });

  it("shows a timeout and lifts it, and says why an action failed", async () => {
    const until = new Date(Date.now() + 3_600_000).toISOString();
    const server = await render(<Member />, ["TIMEOUT_MEMBERS"], (fake) => {
      fake.answer(`GET ${base}/members`, {
        status: 200,
        body: bookClubMembers.map((member) =>
          member.user.id === grace ? { ...member, timedOutUntil: until } : member,
        ),
      });
    });
    server.answer(`DELETE ${base}/members/${grace}/timeout`, {
      status: 403,
      body: { code: "missing_permission", message: "no" },
    });
    await fireEvent.press(await screen.findByTestId("member-timeout-end"));
    expect(await screen.findByText("You don't have the permission for this.")).toBeOnTheScreen();
    expect(screen.queryByTestId("member-kick")).toBeNull();
  });

  it("is not offered for oneself, or without the permissions", async () => {
    params({ id: serverId, userId: ada.id });
    await render(<Member />);
    await screen.findByText("Roles");
    expect(screen.queryByText("Moderation")).toBeNull();
  });
});

describe("bans", () => {
  it("are listed and lifted", async () => {
    params({ id: serverId });
    const ban = {
      user: { id: "troll", username: "troll", displayName: "Troll" },
      reason: "raids",
      createdAt: "2026-11-20T12:00:00Z",
    };
    const server = await render(<Bans />, moderator, (fake) => {
      fake.answer(`GET ${base}/bans`, { status: 200, body: [ban] });
    });
    server.answer(`DELETE ${base}/bans/troll`, { status: 204 });
    expect(await screen.findByText("Reason: ⁨raids⁩")).toBeOnTheScreen();
    server.answer(`GET ${base}/bans`, { status: 200, body: [] });
    await fireEvent.press(screen.getByTestId("ban-lift-troll"));
    expect(await screen.findByText("Nobody is banned.")).toBeOnTheScreen();
  });
});

describe("the audit log", () => {
  const entry = (id: number, action: string, extra: Partial<AuditEntry> = {}): AuditEntry => ({
    id,
    action,
    actorId: ada.id,
    details: {},
    createdAt: "2026-11-20T12:00:00Z",
    ...extra,
  });

  it("reads as sentences, a kind at a time, a page at a time", async () => {
    params({ id: serverId });
    const page = Array.from({ length: 50 }, (_, index) => entry(100 - index, "channel.order"));
    const server = await render(<AuditLog />, moderator, (fake) => {
      fake.answer(`GET ${base}/audit-log`, {
        status: 200,
        body: [
          entry(200, "member.ban", { targetKind: "member", targetId: grace, details: { reason: "raids" } }),
          entry(199, "role.create", { targetKind: "role", targetId: "r1", details: { name: "Mods" } }),
          entry(198, "something.new", { actorId: "gone" }),
          ...page.slice(3),
        ],
      });
    });
    expect(await screen.findByText(/^⁨Ada⁩ banned ⁨Grace⁩$/)).toBeOnTheScreen();
    expect(screen.getByText("Reason: ⁨raids⁩")).toBeOnTheScreen();
    expect(screen.getByText("⁨Ada⁩ created the role ⁨Mods⁩")).toBeOnTheScreen();
    expect(screen.getByText("Someone: something.new ")).toBeOnTheScreen();

    server.answer(`GET ${base}/audit-log`, { status: 200, body: [entry(10, "invite.create")] });
    await fireEvent.press(screen.getByTestId("audit-older"));
    expect(await screen.findByText("⁨Ada⁩ made an invite")).toBeOnTheScreen();
    expect(requestsTo(server, "GET", `${base}/audit-log`).at(-1)?.search).toContain("before=");

    server.answer(`GET ${base}/audit-log`, { status: 200, body: [] });
    await fireEvent.press(screen.getByTestId("audit-kind-4"));
    expect(await screen.findByText("Nothing here yet.")).toBeOnTheScreen();
    expect(requestsTo(server, "GET", `${base}/audit-log`).at(-1)?.search).toContain("action=member");
  });
});

describe("server settings", () => {
  it("lead to reports, bans and the audit log for those who may see them", async () => {
    params({ id: serverId });
    await render(<ServerSettings />);
    await fireEvent.press(await screen.findByTestId("manage-reports"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/server/[id]/reports", params: { id: serverId } });
    expect(screen.getByTestId("manage-bans")).toBeOnTheScreen();
    expect(screen.getByTestId("manage-audit-log")).toBeOnTheScreen();
  });
});
