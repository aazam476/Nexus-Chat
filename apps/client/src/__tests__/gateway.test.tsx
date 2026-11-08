import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { act } from "react";
import { router } from "expo-router";

import Home from "../../app/home";
import { GatewayClient, type SocketLike } from "../realtime/gatewayClient";
import { apply } from "../realtime/useGateway";
import { fakeServer } from "../testing/fakeServer";
import { ada, fakeConnection, renderWithApp } from "../testing/renderWithApp";
import { isolate } from "../i18n/bidi";

/** A socket the test drives: it records what the client sends. */
class FakeSocket implements SocketLike {
  sent: { type: string; data: Record<string, unknown> }[] = [];
  closedWith: number | undefined;
  onmessage: ((message: { data: unknown }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  constructor(readonly url: string) {}
  send(data: string): void {
    this.sent.push(JSON.parse(data) as { type: string; data: Record<string, unknown> });
  }
  close(code?: number): void {
    this.closedWith = code;
  }
  receive(frame: unknown): void {
    this.onmessage?.({ data: typeof frame === "string" ? frame : JSON.stringify(frame) });
  }
  drop(code: number): void {
    this.onclose?.({ code });
  }
}

/** The socket at `index`; fails the test clearly when there is none. */
function nth(sockets: FakeSocket[], index: number): FakeSocket {
  const socket = sockets[index];
  if (socket === undefined) {
    throw new Error(`no socket ${String(index)} was opened`);
  }
  return socket;
}

function startClient(overrides: { onSignedOut?: () => void } = {}) {
  const sockets: FakeSocket[] = [];
  const events: unknown[] = [];
  const client = new GatewayClient({
    address: "https://chat.example.com",
    token: "nxs_t",
    onEvent: (event) => events.push(event),
    onSignedOut: overrides.onSignedOut ?? jest.fn(),
    openSocket: (url) => {
      const socket = new FakeSocket(url);
      sockets.push(socket);
      return socket;
    },
    random: () => 0.5,
  });
  client.start();
  return { client, sockets, events };
}

describe("GatewayClient", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it("identifies, delivers events and sends heartbeats", () => {
    const { client, sockets, events } = startClient();
    const socket = nth(sockets, 0);
    expect(socket.url).toBe("wss://chat.example.com/api/v1/gateway");

    socket.receive({ type: "hello", data: { heartbeatIntervalMs: 1000 } });
    socket.receive({ type: "ready", seq: 1, data: { gatewaySessionId: "s1" } });
    socket.receive({ type: "presence_update", seq: 2, data: { status: "online" } });
    socket.receive("not json");
    socket.receive({ type: "heartbeat_ack", data: {} });
    jest.advanceTimersByTime(1000);
    client.reportActivity(true);

    expect(socket.sent.map((frame) => frame.type)).toEqual(["identify", "heartbeat", "activity"]);
    expect(socket.sent[0]?.data).toEqual({ token: "nxs_t" });
    expect(socket.sent[1]?.data).toEqual({ lastSequence: 2 });
    expect(events).toEqual([
      { type: "ready", seq: 1, data: { gatewaySessionId: "s1" } },
      { type: "presence_update", seq: 2, data: { status: "online" } },
    ]);
  });

  it("reconnects with backoff and resumes where it was", () => {
    const { sockets } = startClient();
    nth(sockets, 0).receive({ type: "hello", data: { heartbeatIntervalMs: 1000 } });
    nth(sockets, 0).receive({ type: "ready", seq: 1, data: { gatewaySessionId: "s1" } });
    nth(sockets, 0).receive({ type: "presence_update", seq: 4, data: {} });

    nth(sockets, 0).drop(1006);
    jest.advanceTimersByTime(749); // 1 s base, half of it fixed plus half × 0.5 jitter
    expect(sockets).toHaveLength(1);
    jest.advanceTimersByTime(1);
    expect(sockets).toHaveLength(2);

    nth(sockets, 1).receive({ type: "hello", data: { heartbeatIntervalMs: 1000 } });
    expect(nth(sockets, 1).sent[0]).toEqual({
      type: "resume",
      data: { token: "nxs_t", gatewaySessionId: "s1", lastSequence: 4 },
    });
    nth(sockets, 1).receive({ type: "resumed", data: { replayed: 0 } });

    nth(sockets, 1).drop(1006);
    jest.advanceTimersByTime(750);
    expect(sockets).toHaveLength(3);
    nth(sockets, 2).drop(1006);
    jest.advanceTimersByTime(1499);
    expect(sockets).toHaveLength(3); // the second failure waits longer
  });

  it("identifies again when the session cannot be resumed", () => {
    const { sockets } = startClient();
    const socket = nth(sockets, 0);
    socket.receive({ type: "invalid_session", data: {} });

    expect(socket.sent.at(-1)).toEqual({ type: "identify", data: { token: "nxs_t" } });
  });

  it("stops on a refused token and tells the app", () => {
    const onSignedOut = jest.fn();
    const { sockets } = startClient({ onSignedOut });

    nth(sockets, 0).drop(4004);
    jest.advanceTimersByTime(120_000);

    expect(onSignedOut).toHaveBeenCalled();
    expect(sockets).toHaveLength(1);
  });

  it("stays closed once stopped", () => {
    const { client, sockets } = startClient();
    client.stop();
    nth(sockets, 0).drop(1000);
    jest.advanceTimersByTime(120_000);

    expect(nth(sockets, 0).closedWith).toBe(1000);
    expect(sockets).toHaveLength(1);
    client.reportActivity(false); // no socket: nothing to send
  });

  it("caps the backoff at a minute", () => {
    const { sockets } = startClient();
    for (let attempt = 0; attempt < 10; attempt++) {
      nth(sockets, sockets.length - 1).drop(1006);
      jest.advanceTimersByTime(60_000);
    }
    expect(sockets).toHaveLength(11);
  });
});

describe("gateway events", () => {
  it("updates presence and the user", () => {
    const setStatus = jest.fn();
    const setUser = jest.fn();

    apply(
      { type: "ready", seq: 1, data: { user: ada, presence: { status: "online" } } },
      { setStatus, setUser },
    );
    apply({ type: "presence_update", seq: 2, data: { status: "dnd" } }, { setStatus, setUser });
    apply({ type: "presence_update", seq: 3, data: {} }, { setStatus, setUser });
    apply({ type: "user_update", seq: 4, data: { user: ada } }, { setStatus, setUser });
    apply({ type: "something_new", seq: 5, data: {} }, { setStatus, setUser });

    expect(setStatus.mock.calls).toEqual([["online"], ["dnd"]]);
    expect(setUser).toHaveBeenCalledTimes(2);
  });
});

describe("the home screen", () => {
  const sockets: FakeSocket[] = [];
  beforeEach(() => {
    sockets.length = 0;
    (globalThis as { WebSocket?: unknown }).WebSocket = jest.fn((url: string) => {
      const socket = new FakeSocket(url);
      sockets.push(socket);
      return socket;
    });
  });

  it("shows the user, changes status and shows the presence the gateway reports", async () => {
    const server = fakeServer();
    server.answer("PUT /api/v1/users/me/status", { status: 200, body: { ...ada, status: "dnd" } });
    const connection = await renderWithApp(
      <Home />,
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
    );
    expect(screen.getByText(`Welcome, ${isolate("Ada")}`)).toBeOnTheScreen();

    act(() => {
      nth(sockets, 0).receive({ type: "presence_update", seq: 1, data: { status: "dnd" } });
    });
    expect(screen.getByTestId("presence")).toHaveTextContent("Others see you as: Do not disturb");
    await fireEvent.press(screen.getByTestId("status-dnd"));
    await waitFor(() => {
      expect(connection.setUser).toHaveBeenCalledWith({ ...ada, status: "dnd" });
    });

    await fireEvent.press(screen.getByTestId("logout"));
    expect(connection.signOut).toHaveBeenCalled();
  });

  it("shows unknown presences as offline, and keeps the user on failure", async () => {
    const server = fakeServer();
    const connection = await renderWithApp(
      <Home />,
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
    );
    act(() => {
      nth(sockets, 0).receive({ type: "presence_update", seq: 1, data: { status: "mystery" } });
    });
    expect(screen.getByTestId("presence")).toHaveTextContent("Others see you as: Offline");
    await fireEvent.press(screen.getByTestId("status-idle"));
    await waitFor(() => {
      expect(server.requests.at(-1)?.path).toBe("/api/v1/users/me/status");
    });
    expect(connection.setUser).not.toHaveBeenCalled();
  });

  it("sends a signed-out visitor back to the start", async () => {
    await renderWithApp(<Home />, fakeConnection(fakeServer()));

    expect(router.replace).toHaveBeenCalledWith("/");
    expect(sockets).toHaveLength(0);
  });
});
