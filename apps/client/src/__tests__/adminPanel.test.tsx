import type { AdminServer, AdminUser } from "@nexus-chat/api-client";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import Admin from "../../app/admin";
import AdminReports from "../../app/admin-reports";
import AdminServers from "../../app/admin-servers";
import AdminUsers from "../../app/admin-users";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { readyMessenger } from "../testing/messengers";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";
import { FakeInstance } from "../testing/fakeDelivery";

jest.setTimeout(60_000);

const admin = { ...ada, isInstanceAdmin: true };
const requestsTo = (server: FakeServer, method: string, path: string) =>
  server.requests.filter((request) => request.method === method && request.path === path);

async function open(
  ui: React.ReactElement,
  answers: (server: FakeServer) => void,
  user = admin,
): Promise<FakeServer> {
  const server = fakeServer();
  answers(server);
  const messenger = await readyMessenger(new FakeInstance(), ada, "ada-laptop");
  await renderWithApp(ui, fakeConnection(server, { user }), fakeMessaging({ status: "ready", messenger }));
  return server;
}

const troll: AdminUser = {
  id: "troll",
  username: "troll",
  displayName: "Troll",
  email: "troll@example.com",
  isAdmin: false,
  createdAt: "2026-11-01T00:00:00Z",
  suspensionReason: "",
};

describe("the admin panel", () => {
  it("shows the instance's numbers and changes who may sign up and where reports go", async () => {
    const server = await open(<Admin />, (fake) => {
      fake.answer("GET /api/v1/admin/stats", {
        status: 200,
        body: {
          users: 12,
          suspendedUsers: 1,
          activeUsers: 5,
          servers: 2,
          conversations: 7,
          recentMessages: 40,
          openReports: 3,
          attachmentBytes: 2 << 20,
        },
      });
      fake.answer("GET /api/v1/admin/registration", { status: 200, body: { mode: "invite_only" } });
      fake.answer("PUT /api/v1/admin/registration", { status: 204 });
      fake.answer("GET /api/v1/admin/moderation", { status: 200, body: { reportsToAdmins: false } });
      fake.answer("PUT /api/v1/admin/moderation", { status: 204 });
    });
    expect(await screen.findByText("12 accounts (1 suspended)")).toBeOnTheScreen();
    expect(screen.getByText("3 open reports")).toBeOnTheScreen();
    await fireEvent.press(await screen.findByTestId("registration-mode-2"));
    await fireEvent.press(await screen.findByTestId("reports-to-admins-1"));
    await waitFor(() => {
      expect(requestsTo(server, "PUT", "/api/v1/admin/registration")[0]?.body).toEqual({ mode: "closed" });
      expect(requestsTo(server, "PUT", "/api/v1/admin/moderation")[0]?.body).toEqual({
        reportsToAdmins: true,
      });
    });
    await fireEvent.press(screen.getByTestId("admin-users"));
    expect(router.push).toHaveBeenCalledWith("/admin-users");
  });

  it("saves the federation policy, and says when a list is wrong", async () => {
    const server = await open(<Admin />, (fake) => {
      fake.answer("GET /api/v1/admin/federation", {
        status: 200,
        body: { mode: "open", allowed: [], blocked: ["spam.example"] },
      });
      fake.answer("PUT /api/v1/admin/federation", { status: 422, body: { code: "invalid", message: "no" } });
    });
    await fireEvent.press(await screen.findByTestId("federation-mode-1"));
    await fireEvent.changeText(screen.getByTestId("federation-allowed"), "friends.example\nnot a domain");
    await fireEvent.press(screen.getByTestId("federation-save"));
    expect(await screen.findByText(/^Check the lists/)).toBeOnTheScreen();
    expect(requestsTo(server, "PUT", "/api/v1/admin/federation")[0]?.body).toEqual({
      mode: "allowlist",
      allowed: ["friends.example", "not", "a", "domain"],
      blocked: ["spam.example"],
    });
    server.answer("PUT /api/v1/admin/federation", { status: 204 });
    await fireEvent.press(screen.getByTestId("federation-save"));
    expect(await screen.findByText("Saved.")).toBeOnTheScreen();
  });

  it("sends anyone else away", async () => {
    await open(<AdminUsers />, () => undefined, ada);
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/");
    });
  });
});

describe("accounts", () => {
  it("are found, suspended, reinstated and made admins", async () => {
    const suspended = { ...troll, suspendedAt: "2026-11-20T00:00:00Z", suspensionReason: "spam" };
    const server = await open(<AdminUsers />, (fake) => {
      fake.answer("GET /api/v1/admin/users", {
        status: 200,
        body: [troll, { ...troll, id: ada.id, username: "ada", isAdmin: true }],
      });
      fake.answer("PUT /api/v1/admin/users/troll/suspension", { status: 204 });
      fake.answer("PUT /api/v1/admin/users/troll/admin", {
        status: 422,
        body: { code: "last_admin", message: "no" },
      });
    });
    expect(await screen.findByText("@⁨troll⁩ · ⁨troll@example.com⁩")).toBeOnTheScreen();
    expect(screen.queryByTestId(`user-suspend-${ada.id}`)).toBeNull();

    await fireEvent.changeText(screen.getByTestId("user-reason-troll"), " spam ");
    server.answer("GET /api/v1/admin/users", { status: 200, body: [suspended] });
    await fireEvent.press(screen.getByTestId("user-suspend-troll"));
    expect(await screen.findByText("Suspended: ⁨spam⁩")).toBeOnTheScreen();
    expect(requestsTo(server, "PUT", "/api/v1/admin/users/troll/suspension")[0]?.body).toEqual({
      reason: "spam",
    });

    server.answer("DELETE /api/v1/admin/users/troll/suspension", { status: 204 });
    await fireEvent.press(screen.getByTestId("user-reinstate-troll"));
    await waitFor(() => {
      expect(requestsTo(server, "DELETE", "/api/v1/admin/users/troll/suspension")).toHaveLength(1);
    });
    await fireEvent.press(screen.getByTestId("user-admin-troll"));
    expect(
      await screen.findByText("The instance needs at least one admin who isn't suspended."),
    ).toBeOnTheScreen();

    server.answer("GET /api/v1/admin/users", { status: 200, body: [] });
    await fireEvent.changeText(screen.getByTestId("admin-users-search"), "nobody");
    expect(await screen.findByText("No accounts match.")).toBeOnTheScreen();
    expect(requestsTo(server, "GET", "/api/v1/admin/users").at(-1)?.search).toBe("?q=nobody");
  });
});

describe("servers", () => {
  it("are listed and deleted", async () => {
    const club: AdminServer = {
      id: "club",
      name: "Book club",
      ownerId: ada.id,
      ownerUsername: "ada",
      members: 3,
      createdAt: "2026-11-01T00:00:00Z",
    };
    const server = await open(<AdminServers />, (fake) => {
      fake.answer("GET /api/v1/admin/servers", { status: 200, body: [club] });
      fake.answer("DELETE /api/v1/admin/servers/club", { status: 204 });
    });
    expect(await screen.findByText("3 members · owned by @⁨ada⁩")).toBeOnTheScreen();
    server.answer("GET /api/v1/admin/servers", { status: 200, body: [] });
    await fireEvent.press(screen.getByTestId("admin-server-delete-club"));
    await fireEvent.press(screen.getByTestId("admin-server-delete-club-confirm"));
    expect(await screen.findByText("No servers match.")).toBeOnTheScreen();
  });
});

describe("reports for the admins", () => {
  it("are listed and resolved", async () => {
    const report = {
      id: "r1",
      messageId: "m1",
      evidence: "AA==",
      receipt: "AA==",
      receiptSignature: "AA==",
      comment: "",
      status: "open",
      escalated: true,
      resolution: "",
      createdAt: "2026-11-20T12:00:00Z",
    };
    const server = await open(<AdminReports />, (fake) => {
      fake.answer("GET /api/v1/admin/reports", { status: 200, body: [report] });
      fake.answer("POST /api/v1/admin/reports/r1/resolve", { status: 204 });
    });
    expect(await screen.findByText(/^This report could not be verified/)).toBeOnTheScreen();
    expect(screen.queryByTestId("report-escalate-r1")).toBeNull();
    await fireEvent.press(screen.getByTestId("report-resolve-r1"));
    await waitFor(() => {
      expect(requestsTo(server, "POST", "/api/v1/admin/reports/r1/resolve")[0]?.body).toEqual({
        resolution: "",
      });
    });
  });
});
