import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import ConversationScreen from "../../app/conversation/[id]";
import GroupSettingsScreen from "../../app/group/[id]";
import NewGroup from "../../app/new-group";
import { isolate } from "../i18n/bidi";
import type { Connection } from "../connection/useConnectionState";
import { pickSquareImage } from "../platform/pickImage";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

jest.mock("../platform/pickImage", () => ({ pickSquareImage: jest.fn() }));

const person = (username: string, displayName: string) => ({
  user: { id: `id-${username}`, username, displayName },
  since: "2026-11-12T12:00:00Z",
});

const group = {
  id: "group",
  kind: "group",
  name: "Book club",
  mlsGroupId: "AAAAAAAAAAAAAAAAAAAAAA==",
  createdAt: "2026-11-12T12:00:00Z",
  members: [
    { id: ada.id, username: "ada", displayName: "Ada" },
    { id: "id-grace", username: "grace", displayName: "Grace" },
    { id: "id-alan", username: "alan", displayName: "Alan" },
  ],
};

function serverWithFriends(...friends: ReturnType<typeof person>[]): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/friends", { status: 200, body: { friends, incoming: [], outgoing: [] } });
  server.answer("GET /api/v1/conversations/group", { status: 200, body: group });
  return server;
}

/** `connection` on an instance whose group DMs hold at most `maximum` people. */
function withGroupLimit(connection: Connection, maximum: number): Connection {
  const instance = connection.instance;
  if (instance === undefined) {
    throw new Error("the fake connection has an instance");
  }
  return {
    ...connection,
    instance: { ...instance, info: { ...instance.info, limits: { groupDmMaxMembers: maximum } } },
  };
}

const requests = (server: FakeServer, method: string, path: string) =>
  server.requests.filter((request) => request.method === method && request.path === path);

describe("starting a group", () => {
  it("picks at least two friends, names the group and opens it", async () => {
    const server = serverWithFriends(
      person("grace", "Grace"),
      person("alan", "Alan"),
      person("edsger", "Edsger"),
    );
    server.answer("POST /api/v1/conversations/groups", { status: 201, body: group });
    await renderWithApp(<NewGroup />, fakeConnection(server, { user: ada }));

    await fireEvent.press(await screen.findByTestId("choose-grace"));
    expect(screen.getByTestId("group-create")).toBeDisabled();
    await fireEvent.press(screen.getByTestId("choose-alan"));
    await fireEvent.press(screen.getByTestId("choose-edsger"));
    await fireEvent.press(screen.getByTestId("choose-edsger"));
    expect(screen.getByText("2 of 24 selected")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("group-name"), "  Book club ");
    await fireEvent.press(screen.getByTestId("group-create"));

    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith({
        pathname: "/conversation/[id]",
        params: { id: "group" },
      });
    });
    expect(requests(server, "POST", "/api/v1/conversations/groups")[0]?.body).toEqual({
      memberIds: ["id-grace", "id-alan"],
      name: "Book club",
    });
  });

  it("stops at the instance's limit and explains a refusal", async () => {
    const server = serverWithFriends(
      person("grace", "Grace"),
      person("alan", "Alan"),
      person("edsger", "Edsger"),
    );
    server.answer("POST /api/v1/conversations/groups", {
      status: 403,
      body: { code: "dm_not_allowed", message: "no" },
    });
    await renderWithApp(<NewGroup />, withGroupLimit(fakeConnection(server, { user: ada }), 3));

    await fireEvent.press(await screen.findByTestId("choose-grace"));
    await fireEvent.press(screen.getByTestId("choose-alan"));
    expect(screen.getByTestId("choose-edsger")).toBeDisabled();
    await fireEvent.press(screen.getByTestId("group-create"));
    expect(await screen.findByText(/doesn't accept group messages from you/)).toBeOnTheScreen();
    expect(requests(server, "POST", "/api/v1/conversations/groups")[0]?.body).toEqual({
      memberIds: ["id-grace", "id-alan"],
    });
    await fireEvent.press(screen.getByTestId("new-group-back"));
    expect(router.back).toHaveBeenCalled();
  });

  it("says when there are no friends to pick", async () => {
    await renderWithApp(<NewGroup />, fakeConnection(serverWithFriends(), { user: ada }));
    expect(await screen.findByText(/Add friends first/)).toBeOnTheScreen();
  });
});

describe("a group conversation", () => {
  it("leads to the group's settings, and shows its icon in the list", async () => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "group" };
    const server = serverWithFriends();
    const withIcon = { ...group, iconUrl: "/api/v1/conversation-icons/i1" };
    server.answer("GET /api/v1/conversations/group", { status: 200, body: withIcon });
    server.answer("GET /api/v1/conversations", { status: 200, body: [withIcon] });
    await renderWithApp(<ConversationScreen />, fakeConnection(server, { user: ada }));
    await fireEvent.press(await screen.findByTestId("open-group-settings"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/group/[id]", params: { id: "group" } });
  });
});

describe("group settings", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "group" };
  });

  it("renames, sets and removes the icon, adds a friend, and leaves", async () => {
    const server = serverWithFriends(person("grace", "Grace"), person("edsger", "Edsger"));
    const withIcon = { ...group, iconUrl: "/api/v1/conversation-icons/i1" };
    server.answer("GET /api/v1/conversations/group", { status: 200, body: withIcon });
    server.answer("PATCH /api/v1/conversations/group", { status: 200, body: withIcon });
    server.answer("PUT /api/v1/conversations/group/icon", { status: 200, body: withIcon });
    server.answer("DELETE /api/v1/conversations/group/icon", { status: 200, body: group });
    server.answer("POST /api/v1/conversations/group/members", { status: 204 });
    server.answer("DELETE /api/v1/conversations/group/members/me", { status: 204 });
    jest
      .mocked(pickSquareImage)
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(new Blob(["png"]));
    await renderWithApp(<GroupSettingsScreen />, fakeConnection(server, { user: ada }));

    expect(await screen.findByText(isolate("Alan"))).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("group-rename"), "Compilers");
    await fireEvent.press(screen.getByTestId("group-save-name"));
    await waitFor(() => {
      expect(requests(server, "PATCH", "/api/v1/conversations/group")[0]?.body).toEqual({
        name: "Compilers",
      });
    });
    await fireEvent.press(screen.getByTestId("group-choose-icon")); // cancelled: nothing sent
    await waitFor(() => {
      expect(screen.getByTestId("group-choose-icon")).toBeEnabled();
    });
    await fireEvent.press(screen.getByTestId("group-choose-icon"));
    await waitFor(() => {
      expect(requests(server, "PUT", "/api/v1/conversations/group/icon")).toHaveLength(1);
    });
    await fireEvent.press(screen.getByTestId("group-remove-icon"));
    await waitFor(() => {
      expect(requests(server, "DELETE", "/api/v1/conversations/group/icon")).toHaveLength(1);
    });
    expect(screen.queryByTestId("add-grace")).toBeNull();
    await fireEvent.press(screen.getByTestId("add-edsger"));
    await waitFor(() => {
      expect(requests(server, "POST", "/api/v1/conversations/group/members")[0]?.body).toEqual({
        userId: "id-edsger",
      });
    });

    await fireEvent.press(screen.getByTestId("group-leave"));
    expect(screen.getByText(/Leave\?/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("group-leave"));
    await waitFor(() => {
      expect(router.replace).toHaveBeenCalledWith("/conversations");
    });
  });

  it("explains a failed change and says when the group is full", async () => {
    const server = serverWithFriends(person("edsger", "Edsger"));
    server.answer("PATCH /api/v1/conversations/group", {
      status: 422,
      body: { code: "invalid_name", message: "no" },
    });
    server.answer("PUT /api/v1/conversations/group/icon", {
      status: 429,
      body: { code: "rate_limited", message: "no" },
    });
    jest.mocked(pickSquareImage).mockResolvedValueOnce(new Blob(["png"]));
    await renderWithApp(<GroupSettingsScreen />, withGroupLimit(fakeConnection(server, { user: ada }), 3));

    expect(await screen.findByText("The group is full.")).toBeOnTheScreen();
    expect(screen.queryByTestId("add-edsger")).toBeNull();
    await fireEvent.press(screen.getByTestId("group-save-name"));
    expect(await screen.findByText(/1 to 100 characters/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("group-choose-icon"));
    expect(await screen.findByText(/Too many changes/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("group-back"));
    expect(router.back).toHaveBeenCalled();
  });
});
