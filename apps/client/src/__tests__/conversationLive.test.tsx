import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import ConversationScreen from "../../app/conversation/[id]";
import Conversations from "../../app/conversations";
import { isolate } from "../i18n/bidi";
import type { FakeServer } from "../testing/fakeServer";
import { fakeSockets, nth } from "../testing/fakeSocket";
import { dm, grace, messengersInDm, renderAs, serverWith } from "../testing/messengers";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";

jest.setTimeout(60_000);

describe("typing in a conversation", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  });
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("shows who is typing, tells the others, and stops when their message arrives", async () => {
    const sockets = fakeSockets();
    let now = Date.UTC(2026, 10, 12, 12, 0, 0);
    jest.spyOn(Date, "now").mockImplementation(() => now);
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await renderWithApp(
      <ConversationScreen />,
      fakeConnection(serverWith([dm]), { user: ada, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: adaMessenger }),
    );
    await screen.findByText(/Messages here are end-to-end encrypted/);
    const socket = nth(sockets, 0);
    const typing = (userId: string, conversationId = "dm") => {
      socket.receive({ type: "typing_start", seq: 1, data: { conversationId, userId } });
    };

    await act(async () => {
      typing(grace.id);
      typing(grace.id, "elsewhere");
      typing(ada.id);
      await Promise.resolve();
    });
    expect(screen.getByTestId("typing")).toHaveTextContent(`${isolate("Grace")} is typing…`);

    await fireEvent.changeText(screen.getByTestId("composer"), "h");
    await fireEvent.changeText(screen.getByTestId("composer"), "hi");
    expect(socket.sent.filter((frame) => frame.type === "typing")).toEqual([
      { type: "typing", data: { conversationId: "dm" } },
    ]);

    await graceMessenger.send("dm", { kind: "text", text: "done typing" });
    await act(async () => {
      await adaMessenger.sync("dm");
    });
    expect(await screen.findByText("done typing")).toBeOnTheScreen();
    await waitFor(() => {
      expect(screen.getByTestId("typing")).toHaveTextContent("");
    });

    await act(async () => {
      typing(grace.id);
      await Promise.resolve();
    });
    expect(screen.getByTestId("typing")).toHaveTextContent(/is typing/);
    now += 9000;
    await waitFor(
      () => {
        expect(screen.getByTestId("typing")).toHaveTextContent("");
      },
      { timeout: 3000 },
    );
  });

  it("names two typists, then says several", async () => {
    const { ada: adaMessenger } = await messengersInDm();
    const sockets = fakeSockets();
    const group = {
      ...dm,
      members: [
        ...dm.members,
        { id: "alan", username: "alan", displayName: "Alan" },
        { id: "edsger", username: "edsger", displayName: "Edsger" },
      ],
    };
    const server = serverWith([group]);
    server.answer("GET /api/v1/conversations/dm", { status: 200, body: group });
    await renderWithApp(
      <ConversationScreen />,
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: adaMessenger }),
    );
    await screen.findByText(/Messages here are end-to-end encrypted/);
    const socket = nth(sockets, 0);
    await act(async () => {
      socket.receive({ type: "typing_start", seq: 1, data: { conversationId: "dm", userId: grace.id } });
      socket.receive({ type: "typing_start", seq: 2, data: { conversationId: "dm", userId: "alan" } });
      await Promise.resolve();
    });
    expect(screen.getByTestId("typing")).toHaveTextContent(
      `${isolate("Grace")} and ${isolate("Alan")} are typing…`,
    );
    await act(async () => {
      socket.receive({ type: "typing_start", seq: 3, data: { conversationId: "dm", userId: "edsger" } });
      socket.receive({ type: "typing_start", seq: 4, data: null });
      await Promise.resolve();
    });
    expect(screen.getByTestId("typing")).toHaveTextContent("Several people are typing…");
  });
});

describe("read state", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  });

  const readState = (server: FakeServer, lastReadId: number, unreadCount: number): void => {
    server.answer("GET /api/v1/read-state", {
      status: 200,
      body: [{ conversationId: "dm", lastReadId, unreadCount }],
    });
  };
  const requests = (server: FakeServer, method: string, path: string) =>
    server.requests.filter((request) => request.method === method && request.path === path);

  it("shows each conversation's unread count in the list", async () => {
    const { ada: adaMessenger } = await messengersInDm();
    const server = serverWith([dm]);
    readState(server, 0, 3);
    await renderAs(<Conversations />, adaMessenger, server);
    expect(await screen.findByLabelText("3 unread messages")).toBeOnTheScreen();
  });

  it("marks where new messages begin, jumps there, and marks them read", async () => {
    const sockets = fakeSockets();
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await graceMessenger.send("dm", { kind: "text", text: "seen before" });
    await graceMessenger.send("dm", { kind: "text", text: "first new one" });
    await graceMessenger.send("dm", { kind: "text", text: "second new one" });
    await adaMessenger.sync("dm");
    const [seen] = await adaMessenger.messages("dm");
    const server = serverWith([dm]);
    readState(server, seen?.deliveryId ?? 0, 2);
    server.answer("PUT /api/v1/conversations/dm/read", {
      status: 200,
      body: { conversationId: "dm", lastReadId: 99, unreadCount: 0 },
    });
    await renderWithApp(
      <ConversationScreen />,
      fakeConnection(server, { user: ada, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: adaMessenger }),
    );

    expect(await screen.findByTestId("new-messages")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("jump-to-unread"));
    await waitFor(() => {
      expect(requests(server, "PUT", "/api/v1/conversations/dm/read")).toHaveLength(1);
    });
    const newest = (await adaMessenger.messages("dm")).at(-1)?.deliveryId;
    expect(requests(server, "PUT", "/api/v1/conversations/dm/read")[0]?.body).toEqual({ lastReadId: newest });

    // Another device read further: the state is fetched again.
    const before = requests(server, "GET", "/api/v1/read-state").length;
    await act(async () => {
      nth(sockets, 0).receive({
        type: "read_update",
        seq: 1,
        data: { conversationId: "dm", lastReadId: 99 },
      });
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(requests(server, "GET", "/api/v1/read-state").length).toBeGreaterThan(before);
    });
  });

  it("marks nothing read and shows no marker when everything was read", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await graceMessenger.send("dm", { kind: "text", text: "old news" });
    await adaMessenger.sync("dm");
    const newest = (await adaMessenger.messages("dm")).at(-1)?.deliveryId ?? 0;
    const server = serverWith([dm]);
    readState(server, newest, 0);
    await renderAs(<ConversationScreen />, adaMessenger, server);
    await screen.findByText("old news");
    await waitFor(() => {
      expect(requests(server, "GET", "/api/v1/read-state").length).toBeGreaterThan(0);
    });
    expect(screen.queryByTestId("new-messages")).toBeNull();
    expect(screen.queryByTestId("jump-to-unread")).toBeNull();
    expect(requests(server, "PUT", "/api/v1/conversations/dm/read")).toHaveLength(0);
  });
});
