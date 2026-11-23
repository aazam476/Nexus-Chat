import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import ChannelScreen from "../../app/server/[id]/channel/[channelId]";
import ServerScreen from "../../app/server/[id]/index";
import Servers from "../../app/servers";
import { isolate } from "../i18n/bidi";
import { fakeServer } from "../testing/fakeServer";
import { fakeSockets, nth } from "../testing/fakeSocket";
import { ada as adaUser, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";
import {
  audienceId,
  answerServer,
  bookClub,
  general,
  messengersInServer,
  serverId,
} from "../testing/servers";
import { audiencesOf } from "../servers/audiences";

jest.setTimeout(60_000);

const renamed = (name: string) => {
  const details = bookClub();
  return { ...details, server: { ...details.server, name } };
};

describe("servers as they change", () => {
  it("reload when the gateway says the server changed", async () => {
    const sockets = fakeSockets();
    const { ada: adaMessenger } = await messengersInServer();
    const server = fakeServer();
    answerServer(server);
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId };
    await renderWithApp(
      <ServerScreen />,
      fakeConnection(server, { user: adaUser, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: adaMessenger }),
    );
    expect(await screen.findByText(isolate("Book club"))).toBeOnTheScreen();

    answerServer(server, renamed("Reading room"));
    await act(async () => {
      const socket = nth(sockets, 0);
      socket.receive({ type: "server_update", seq: 1, data: { serverId: "another" } });
      socket.receive({ type: "server_update", seq: 2, data: null });
      socket.receive({ type: "server_update", seq: 3, data: { serverId } });
      await Promise.resolve();
    });
    expect(await screen.findByText(isolate("Reading room"))).toBeOnTheScreen();
  });

  it("reload the list too", async () => {
    const sockets = fakeSockets();
    const { ada: adaMessenger } = await messengersInServer();
    const server = fakeServer();
    answerServer(server);
    await renderWithApp(
      <Servers />,
      fakeConnection(server, { user: adaUser, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: adaMessenger }),
    );
    expect(await screen.findByText(isolate("Book club"))).toBeOnTheScreen();
    answerServer(server, renamed("Reading room"));
    await act(async () => {
      nth(sockets, 0).receive({ type: "server_update", seq: 1, data: { serverId } });
      await Promise.resolve();
    });
    expect(await screen.findByText(isolate("Reading room"))).toBeOnTheScreen();
  });
});

describe("channels as they are read", () => {
  it("show unread counts that update when the user reads elsewhere", async () => {
    const sockets = fakeSockets();
    const { ada } = await messengersInServer();
    const server = fakeServer();
    answerServer(server);
    server.answer(`GET /api/v1/servers/${serverId}/read-state`, {
      status: 200,
      body: [{ channelId: general, lastReadId: 0, unreadCount: 3 }],
    });
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId };
    await renderWithApp(
      <ServerScreen />,
      fakeConnection(server, { user: adaUser, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: ada }),
    );
    expect(await screen.findByLabelText("3 unread messages")).toBeOnTheScreen();
    server.answer(`GET /api/v1/servers/${serverId}/read-state`, {
      status: 200,
      body: [{ channelId: general, lastReadId: 9, unreadCount: 0 }],
    });
    await act(async () => {
      nth(sockets, 0).receive({ type: "read_update", seq: 1, data: { channelId: general, lastReadId: 9 } });
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(screen.queryByLabelText("3 unread messages")).toBeNull();
    });
  });

  it("marks a channel read as its messages show, and shares typing there", async () => {
    const sockets = fakeSockets();
    const { ada, grace } = await messengersInServer();
    const [audience] = audiencesOf(bookClub(), []);
    if (audience !== undefined) {
      await grace.sendInChannel(audience, general, { kind: "text", text: "unread one" });
      await ada.sync(audienceId);
    }
    const server = fakeServer();
    answerServer(server);
    server.answer(`GET /api/v1/servers/${serverId}/read-state`, {
      status: 200,
      body: [{ channelId: general, lastReadId: 0, unreadCount: 1 }],
    });
    server.answer(`PUT /api/v1/servers/${serverId}/channels/${general}/read`, {
      status: 200,
      body: { channelId: general, lastReadId: 1, unreadCount: 0 },
    });
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: serverId, channelId: general };
    await renderWithApp(
      <ChannelScreen />,
      fakeConnection(server, { user: adaUser, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: ada }),
    );
    expect(await screen.findByText("unread one")).toBeOnTheScreen();
    await waitFor(() => {
      expect(
        server.requests.some((request) => request.method === "PUT" && request.path.endsWith("/read")),
      ).toBe(true);
    });
    await fireEvent.changeText(screen.getByTestId("composer"), "typing here");
    expect(nth(sockets, 0).sent.filter((frame) => frame.type === "typing")).toEqual([
      { type: "typing", data: { channelId: general } },
    ]);
  });
});
