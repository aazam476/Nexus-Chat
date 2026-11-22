import { act, screen } from "@testing-library/react-native";

import ServerScreen from "../../app/server/[id]/index";
import Servers from "../../app/servers";
import { isolate } from "../i18n/bidi";
import { fakeServer } from "../testing/fakeServer";
import { fakeSockets, nth } from "../testing/fakeSocket";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";
import { answerServer, bookClub, messengersInServer, serverId } from "../testing/servers";

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
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
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
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
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
