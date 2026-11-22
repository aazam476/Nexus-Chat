import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { Share } from "react-native";

import Invites from "../../app/server/[id]/invites";
import { fakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);

const invite = (code: string, extra: object = {}) => ({
  code,
  serverId,
  uses: 0,
  createdAt: "2026-11-20T00:00:00Z",
  ...extra,
});

describe("inviting people", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId };
  });
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("makes a link with the chosen limits, shares it, and revokes invites", async () => {
    const share = jest
      .spyOn(Share, "share")
      .mockResolvedValue({ action: "sharedAction", activityType: undefined });
    const { ada } = await messengersInServer();
    const server = fakeServer();
    answerServer(server);
    server.answer(`GET /api/v1/servers/${serverId}/invites`, {
      status: 200,
      body: [invite("older001", { maxUses: 10, uses: 3, expiresAt: "2026-12-01T00:00:00Z" })],
    });
    server.answer(`POST /api/v1/servers/${serverId}/invites`, { status: 201, body: invite("newcode1") });
    server.answer(`DELETE /api/v1/servers/${serverId}/invites/older001`, { status: 204 });
    await renderAs(<Invites />, ada, server);

    expect(await screen.findByText(/Used 3 of 10 times/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("invite-expiry-3"));
    await fireEvent.press(screen.getByTestId("invite-uses-1"));
    await fireEvent.press(screen.getByTestId("invite-create"));
    expect(await screen.findByText("https://chat.example.com/invite/newcode1")).toBeOnTheScreen();
    expect(server.requests.find((request) => request.method === "POST")?.body).toEqual({ maxUses: 1 });
    expect(screen.getByText(/Used 0 times · Never expires/)).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("invite-share"));
    expect(share.mock.calls[0]?.[0]).toMatchObject({
      message: /https:\/\/chat\.example\.com\/invite\/newcode1/,
    });
    await fireEvent.press(screen.getByTestId("invite-revoke-older001"));
    await waitFor(() => {
      expect(screen.queryByText(/Used 3 of 10 times/)).toBeNull();
    });
  });

  it("lets a member without MANAGE_SERVER make links but not list them, and explains a refusal", async () => {
    jest.spyOn(Share, "share").mockRejectedValue(new Error("no share sheet"));
    const { ada } = await messengersInServer();
    const server = fakeServer();
    answerServer(server, bookClub({ permissions: ["VIEW_CHANNEL", "CREATE_INVITES"] }));
    server.answer(`POST /api/v1/servers/${serverId}/invites`, {
      status: 403,
      body: { code: "missing_permission", message: "x" },
    });
    await renderAs(<Invites />, ada, server);
    await fireEvent.press(await screen.findByTestId("invite-create"));
    expect(await screen.findByText("You don't have the permission for this.")).toBeOnTheScreen();
    expect(screen.queryByText("Active invites")).toBeNull();

    server.answer(`POST /api/v1/servers/${serverId}/invites`, { status: 201, body: invite("plain001") });
    await fireEvent.press(screen.getByTestId("invite-create"));
    await fireEvent.press(await screen.findByTestId("invite-share"));
    expect(screen.getByTestId("invite-link")).toHaveTextContent("https://chat.example.com/invite/plain001");
  });
});
