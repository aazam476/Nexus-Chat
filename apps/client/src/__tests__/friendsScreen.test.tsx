import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import FriendsScreen from "../../app/friends";
import { isolate } from "../i18n/bidi";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { dm } from "../testing/messengers";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";

const person = (username: string, displayName: string) => ({
  user: { id: `id-${username}`, username, displayName },
  since: "2026-11-10T12:00:00Z",
});

function friendsServer(): FakeServer {
  const server = fakeServer();
  server.answer("GET /api/v1/friends", {
    status: 200,
    body: {
      friends: [person("grace", "Grace")],
      incoming: [person("alan", "Alan")],
      outgoing: [person("edsger", "Edsger")],
    },
  });
  return server;
}

const calls = (server: FakeServer, method: string, path: string): number =>
  server.requests.filter((request) => request.method === method && request.path === path).length;

describe("the friends screen", () => {
  it("lists friends and requests and acts on them", async () => {
    const server = friendsServer();
    for (const route of [
      "POST /api/v1/friends/requests/id-alan/accept",
      "DELETE /api/v1/friends/requests/id-alan",
      "DELETE /api/v1/friends/requests/id-edsger",
      "DELETE /api/v1/friends/id-grace",
    ]) {
      server.answer(route, { status: 204 });
    }
    await renderWithApp(<FriendsScreen />, fakeConnection(server, { user: ada }));
    expect(await screen.findByText(isolate("Grace"))).toBeOnTheScreen();
    expect(screen.getByText("@alan")).toBeOnTheScreen();
    expect(screen.getByText("Requests you sent")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("accept-alan"));
    await fireEvent.press(screen.getByText("Decline"));
    await fireEvent.press(screen.getByText("Cancel request"));
    await fireEvent.press(screen.getByText("Remove"));
    await waitFor(() => {
      expect(calls(server, "DELETE", "/api/v1/friends/id-grace")).toBe(1);
    });
    expect(calls(server, "POST", "/api/v1/friends/requests/id-alan/accept")).toBe(1);
    expect(calls(server, "DELETE", "/api/v1/friends/requests/id-edsger")).toBe(1);
    // Every change reloads the lists.
    expect(calls(server, "GET", "/api/v1/friends")).toBeGreaterThan(1);
  });

  it("sends a request by username and says whether it went", async () => {
    const server = friendsServer();
    server.answer("POST /api/v1/friends/requests", {
      status: 200,
      body: { userId: "id-ada2", relationship: "outgoing" },
    });
    await renderWithApp(<FriendsScreen />, fakeConnection(server, { user: ada }));
    await fireEvent.press(screen.getByTestId("friend-send"));
    expect(calls(server, "POST", "/api/v1/friends/requests")).toBe(0);

    await fireEvent.changeText(screen.getByTestId("friend-username"), "@ada2 ");
    await fireEvent.press(screen.getByTestId("friend-send"));
    expect(await screen.findByText(`Request sent to ${isolate("ada2")}.`)).toBeOnTheScreen();
    expect(server.requests.find((request) => request.path === "/api/v1/friends/requests")?.body).toEqual({
      username: "ada2",
    });

    server.answer("POST /api/v1/friends/requests", {
      status: 403,
      body: { code: "forbidden", message: "no" },
    });
    await fireEvent.changeText(screen.getByTestId("friend-username"), "nobody");
    await fireEvent(screen.getByTestId("friend-username"), "submitEditing");
    expect(await screen.findByText(/could not be sent/)).toBeOnTheScreen();
  });

  it("opens the DM with a friend, or says it is not allowed", async () => {
    const server = friendsServer();
    server.answer("POST /api/v1/conversations/dms", {
      status: 403,
      body: { code: "forbidden", message: "no" },
    });
    await renderWithApp(<FriendsScreen />, fakeConnection(server, { user: ada }));
    await fireEvent.press(await screen.findByTestId("message-grace"));
    expect(await screen.findByText(/could not be started/)).toBeOnTheScreen();

    server.answer("POST /api/v1/conversations/dms", { status: 200, body: dm });
    await fireEvent.press(screen.getByTestId("message-grace"));
    await waitFor(() => {
      expect(router.push).toHaveBeenCalledWith({ pathname: "/conversation/[id]", params: { id: "dm" } });
    });
    await fireEvent.press(screen.getByTestId("friends-back"));
    expect(router.back).toHaveBeenCalled();
  });

  it("says when there are no friends yet, and sends a logged-out user away", async () => {
    const server = fakeServer();
    server.answer("GET /api/v1/friends", { status: 200, body: { friends: [], incoming: [], outgoing: [] } });
    await renderWithApp(<FriendsScreen />, fakeConnection(server));
    expect(await screen.findByText(/No friends yet/)).toBeOnTheScreen();
    expect(router.replace).toHaveBeenCalledWith("/");
  });
});
