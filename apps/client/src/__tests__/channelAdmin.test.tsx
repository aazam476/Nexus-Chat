import type { ServerChannel } from "@nexus-chat/api-client";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import EditChannel from "../../app/server/[id]/channel/[channelId]/edit";
import ChannelOverride from "../../app/server/[id]/channel/[channelId]/override";
import ChannelAdmin from "../../app/server/[id]/channels";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import {
  answerServer,
  bookClub,
  bookClubMembers,
  general,
  messengersInServer,
  serverId,
} from "../testing/servers";

jest.setTimeout(60_000);

const base = `/api/v1/servers/${serverId}`;
const books: ServerChannel = {
  id: "books",
  kind: "text",
  name: "books",
  topic: "",
  parentId: "cat-text",
  position: 1,
  overrides: [{ targetKind: "role", targetId: "role-everyone", allow: [], deny: ["SEND_MESSAGES"] }],
  permissions: ["VIEW_CHANNEL"],
};
const withBooks = () => {
  const details = bookClub();
  return { ...details, channels: [...details.channels, books] };
};
const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

async function render(ui: React.ReactElement): Promise<FakeServer> {
  const { ada } = await messengersInServer();
  const server = fakeServer();
  answerServer(server, withBooks());
  await renderAs(ui, ada, server);
  return server;
}

const lastBody = (server: FakeServer, method: string) =>
  server.requests.filter((request) => request.method === method).at(-1)?.body;

describe("managing channels", () => {
  beforeEach(() => {
    params({ id: serverId });
  });

  it("moves channels and adds new ones", async () => {
    const server = await render(<ChannelAdmin />);
    server.answer(`PUT ${base}/channel-order`, { status: 204 });
    server.answer(`POST ${base}/channels`, { status: 201, body: { id: "new" } });
    await fireEvent.press(await screen.findByTestId("channel-down-" + general));
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toEqual([
        { id: "voice-1" },
        { id: "cat-text" },
        { id: "books", parentId: "cat-text" },
        { id: general, parentId: "cat-text" },
      ]);
    });
    expect(screen.queryByTestId(`channel-up-${general}`)).toBeNull();
    await fireEvent.press(screen.getByTestId(`channel-edit-${general}`));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]/edit",
      params: { id: serverId, channelId: general },
    });

    await fireEvent.changeText(screen.getByTestId("new-channel-name"), "news");
    await fireEvent.press(screen.getByTestId("new-channel-create"));
    await waitFor(() => {
      expect(lastBody(server, "POST")).toEqual({ kind: "text", name: "news", parentId: "cat-text" });
    });
    await fireEvent.press(screen.getByTestId("new-channel-kind-2"));
    expect(screen.queryByTestId("new-channel-category-0")).toBeNull();
    await fireEvent.changeText(screen.getByTestId("new-channel-name"), "Archive");
    await fireEvent.press(screen.getByTestId("new-channel-create"));
    await waitFor(() => {
      expect(lastBody(server, "POST")).toEqual({ kind: "category", name: "Archive" });
    });
  });

  it("explains a refused move", async () => {
    const server = await render(<ChannelAdmin />);
    server.answer(`PUT ${base}/channel-order`, {
      status: 403,
      body: { code: "missing_permission", message: "x" },
    });
    await fireEvent.press(await screen.findByTestId(`channel-up-books`));
    expect(await screen.findByText("You don't have the permission for this.")).toBeOnTheScreen();
  });
});

describe("editing a channel", () => {
  beforeEach(() => {
    params({ id: serverId, channelId: "books" });
  });

  it("saves its name and topic, moves it, and deletes it", async () => {
    const server = await render(<EditChannel />);
    server.answer(`PATCH ${base}/channels/books`, { status: 204 });
    server.answer(`PUT ${base}/channel-order`, { status: 204 });
    server.answer(`DELETE ${base}/channels/books`, { status: 204 });
    await fireEvent.changeText(await screen.findByTestId("channel-name"), "reading");
    await fireEvent.changeText(screen.getByTestId("channel-topic"), "What we read");
    await fireEvent.press(screen.getByTestId("channel-save"));
    await waitFor(() => {
      expect(lastBody(server, "PATCH")).toEqual({ name: "reading", topic: "What we read" });
    });
    await fireEvent.press(screen.getByTestId("channel-category-0"));
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toContainEqual({ id: "books" });
    });
    await fireEvent.press(screen.getByTestId("channel-delete"));
    await fireEvent.press(screen.getByTestId("channel-delete-confirm"));
    await waitFor(() => {
      expect(router.back).toHaveBeenCalled();
    });
  });

  it("lists its overrides and adds one for a role or a person", async () => {
    await render(<EditChannel />);
    expect(await screen.findByText("−SEND_MESSAGES")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("override-role-everyone"));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]/override",
      params: { id: serverId, channelId: "books", kind: "role", target: "role-everyone" },
    });
    const grace = bookClubMembers[1]?.user.id ?? "";
    await fireEvent.press(screen.getByTestId(`override-add-member-${grace}`));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]/override",
      params: { id: serverId, channelId: "books", kind: "member", target: grace },
    });
  });

  it("says when the channel is gone", async () => {
    params({ id: serverId, channelId: "gone" });
    await render(<EditChannel />);
    expect(await screen.findByText(/This channel is gone/)).toBeOnTheScreen();
  });
});

describe("a channel override", () => {
  it("changes permissions three ways and saves them", async () => {
    params({ id: serverId, channelId: "books", kind: "role", target: "role-everyone" });
    const server = await render(<ChannelOverride />);
    server.answer(`PUT ${base}/channels/books/overrides/role/role-everyone`, { status: 204 });
    expect(await screen.findByText(/Permissions for .*@everyone/)).toBeOnTheScreen();
    expect(screen.getByTestId("perm-SEND_MESSAGES-deny")).toHaveTextContent(/✓ Deny/);
    await fireEvent.press(screen.getByTestId("perm-VIEW_CHANNEL-allow"));
    await fireEvent.press(screen.getByTestId("override-save"));
    await waitFor(() => {
      expect(lastBody(server, "PUT")).toEqual({ allow: ["VIEW_CHANNEL"], deny: ["SEND_MESSAGES"] });
    });
    expect(router.back).toHaveBeenCalled();
    // Ada may not grant what she does not have.
    expect(screen.getByTestId("perm-SPEAK-allow")).toBeDisabled();
  });

  it("removes the override when nothing is allowed or denied", async () => {
    params({ id: serverId, channelId: "books", kind: "role", target: "role-everyone" });
    const server = await render(<ChannelOverride />);
    server.answer(`DELETE ${base}/channels/books/overrides/role/role-everyone`, { status: 204 });
    await fireEvent.press(await screen.findByTestId("perm-SEND_MESSAGES-neutral"));
    await fireEvent.press(screen.getByTestId("override-save"));
    await waitFor(() => {
      expect(server.requests.some((request) => request.method === "DELETE")).toBe(true);
    });
  });
});
