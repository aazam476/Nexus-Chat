import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import ServerSettings from "../../app/server/[id]/settings";
import { pickSquareImage } from "../platform/pickImage";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);
jest.mock("../platform/pickImage", () => ({ pickSquareImage: jest.fn() }));

const base = `/api/v1/servers/${serverId}`;

async function open(details = bookClub()): Promise<FakeServer> {
  const { ada } = await messengersInServer();
  const server = fakeServer();
  answerServer(server, details);
  await renderAs(<ServerSettings />, ada, server);
  return server;
}

describe("server settings", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId };
  });

  it("saves the overview and changes the icon", async () => {
    const server = await open();
    server.answer(`PATCH ${base}`, { status: 204 });
    server.answer(`PUT ${base}/icon`, { status: 204 });
    await fireEvent.changeText(await screen.findByTestId("settings-name"), "Reading room");
    await fireEvent.changeText(screen.getByTestId("settings-description"), "We read more.");
    await fireEvent.press(screen.getByTestId("settings-compress-0"));
    await fireEvent.press(screen.getByTestId("settings-save"));
    expect(await screen.findByText("Saved.")).toBeOnTheScreen();
    expect(server.requests.find((request) => request.method === "PATCH")?.body).toEqual({
      name: "Reading room",
      description: "We read more.",
      compressMedia: true,
    });

    jest
      .mocked(pickSquareImage)
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(new Blob(["png"]));
    await fireEvent.press(screen.getByTestId("server-choose-icon"));
    await fireEvent.press(screen.getByTestId("server-choose-icon"));
    await waitFor(() => {
      expect(
        server.requests.some((request) => request.method === "PUT" && request.path.endsWith("/icon")),
      ).toBe(true);
    });
  });

  it("removes an icon and leads to channels, roles and members", async () => {
    const details = bookClub();
    const server = await open({
      ...details,
      server: { ...details.server, iconUrl: "/api/v1/server-icons/x" },
    });
    server.answer(`DELETE ${base}/icon`, { status: 204 });
    await fireEvent.press(await screen.findByTestId("server-remove-icon"));
    await waitFor(() => {
      expect(server.requests.some((request) => request.method === "DELETE")).toBe(true);
    });
    for (const [testID, pathname] of [
      ["manage-channels", "/server/[id]/channels"],
      ["manage-roles", "/server/[id]/roles"],
      ["manage-members", "/server/[id]/members"],
    ] as const) {
      await fireEvent.press(screen.getByTestId(testID));
      expect(router.push).toHaveBeenCalledWith({ pathname, params: { id: serverId } });
    }
  });

  it("lets the owner delete the server after asking", async () => {
    const server = await open();
    server.answer(`DELETE ${base}`, { status: 204 });
    await fireEvent.press(await screen.findByTestId("server-delete"));
    await fireEvent.press(screen.getByText("Keep it"));
    await fireEvent.press(screen.getByTestId("server-delete"));
    await fireEvent.press(screen.getByTestId("server-delete-confirm"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/servers");
    });
  });

  it("shows a member only what they may manage, and explains refusals", async () => {
    const details = bookClub({ permissions: ["VIEW_CHANNEL", "MANAGE_SERVER"] });
    const server = await open({ ...details, server: { ...details.server, ownerId: "someone" } });
    server.answer(`PATCH ${base}`, { status: 403, body: { code: "missing_permission", message: "x" } });
    await fireEvent.press(await screen.findByTestId("settings-save"));
    expect(await screen.findByText("You don't have the permission for this.")).toBeOnTheScreen();
    expect(screen.queryByTestId("manage-channels")).toBeNull();
    expect(screen.getByTestId("manage-members")).toBeOnTheScreen();
    expect(screen.queryByTestId("server-delete")).toBeNull();
  });
});
