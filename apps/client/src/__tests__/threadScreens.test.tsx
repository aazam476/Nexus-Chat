import type { ServerDetails } from "@nexus-chat/api-client";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import ChannelScreen from "../../app/server/[id]/channel/[channelId]";
import ThreadScreen from "../../app/server/[id]/channel/[channelId]/thread/[rootId]";
import { audiencesOf } from "../servers/audiences";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, general, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);

const base = `/api/v1/servers/${serverId}/channels/${general}/threads`;
const text = (body: string) => ({ kind: "text" as const, text: body });

/** Book club where #general allows `permissions` on top of reading and sending. */
function clubWith(...permissions: ServerDetails["permissions"]): ServerDetails {
  const details = bookClub();
  return {
    ...details,
    channels: details.channels.map((channel) =>
      channel.id === general
        ? { ...channel, permissions: [...channel.permissions, ...permissions] }
        : channel,
    ),
  };
}

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

/** Grace posts in #general, and replies in its thread `replies` times; Ada's screen opens on that. */
async function open(
  ui: React.ReactElement,
  details: ServerDetails,
  replies: string[] = [],
): Promise<{ server: FakeServer; root: string }> {
  const { ada, grace } = await messengersInServer();
  const audience = audiencesOf(bookClub(), [])[0];
  if (audience === undefined) {
    throw new Error("no audience");
  }
  const root = await grace.sendInChannel(audience, general, text("What are we reading?"));
  for (const reply of replies) {
    await grace.sendInChannel(audience, general, text(reply), root);
  }
  await ada.sync(audience.audienceId);
  const server = fakeServer();
  answerServer(server, details);
  server.answer(`GET /api/v1/servers/${serverId}/read-state`, { status: 200, body: [] });
  server.answer(`GET ${base}`, { status: 200, body: [] });
  params({ id: serverId, channelId: general, rootId: root });
  await renderAs(ui, ada, server);
  return { server, root };
}

describe("threads in a channel", () => {
  it("are started from a message, by those who may", async () => {
    const { root } = await open(<ChannelScreen />, clubWith("SEND_IN_THREADS", "CREATE_THREADS"));
    await fireEvent.press(await screen.findByTestId(`message-${root}`));
    await fireEvent.press(screen.getByTestId("message-thread"));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]/thread/[rootId]",
      params: { id: serverId, channelId: general, rootId: root },
    });
  });

  it("show their reply count, and leave the replies out of the channel", async () => {
    const { root } = await open(<ChannelScreen />, clubWith("SEND_IN_THREADS"), ["Dune", "Again?"]);
    const link = await screen.findByTestId(`thread-${root}`);
    expect(link).toHaveTextContent(/^2 replies, last at/);
    expect(screen.queryByText("Dune")).toBeNull();

    // Without CREATE_THREADS, a thread that exists can still be replied in.
    await fireEvent.press(screen.getByTestId(`message-${root}`));
    expect(screen.getByTestId("message-thread")).toBeOnTheScreen();
    await fireEvent.press(link);
    expect(router.push).toHaveBeenCalled();
  });

  it("are not offered without the thread permissions", async () => {
    const { root } = await open(<ChannelScreen />, clubWith("SEND_IN_THREADS"));
    await fireEvent.press(await screen.findByTestId(`message-${root}`));
    expect(screen.queryByTestId("message-thread")).toBeNull();
  });
});

describe("a thread", () => {
  it("shows the message it is on, takes replies, and can be followed", async () => {
    const { server, root } = await open(<ThreadScreen />, clubWith("SEND_IN_THREADS", "CREATE_THREADS"));
    expect(await screen.findByText("What are we reading?")).toBeOnTheScreen();
    expect(screen.getByText("No replies yet. Start the thread below.")).toBeOnTheScreen();
    expect(screen.queryByTestId("thread-follow")).toBeNull();

    server.answer(`GET ${base}`, {
      status: 200,
      body: [{ rootId: root, replyCount: 1, createdAt: "x", lastReplyAt: "x", following: false }],
    });
    await fireEvent.changeText(screen.getByTestId("composer"), "Dune");
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("Dune")).toBeOnTheScreen();
    expect(screen.queryByText("No replies yet. Start the thread below.")).toBeNull();

    server.answer(`PUT ${base}/${root}/follow`, { status: 204 });
    await fireEvent.press(await screen.findByLabelText("Follow"));
    await waitFor(() => {
      expect(
        server.requests.some((request) => request.method === "PUT" && request.path.endsWith("/follow")),
      ).toBe(true);
    });
    server.answer(`GET ${base}`, {
      status: 200,
      body: [{ rootId: root, replyCount: 1, createdAt: "x", lastReplyAt: "x", following: true }],
    });
    server.answer(`DELETE ${base}/${root}/follow`, { status: 204 });
    await fireEvent.changeText(screen.getByTestId("composer"), "Again?");
    await fireEvent.press(screen.getByTestId("composer-send"));
    await fireEvent.press(await screen.findByLabelText("Stop following"));
    await waitFor(() => {
      expect(server.requests.some((request) => request.method === "DELETE")).toBe(true);
    });
  });

  it("shows replies sent from elsewhere, and is read-only without SEND_IN_THREADS", async () => {
    await open(<ThreadScreen />, bookClub(), ["Dune"]);
    expect(await screen.findByText("Dune")).toBeOnTheScreen();
    expect(screen.getByText("You can read this thread but not reply in it.")).toBeOnTheScreen();
    expect(screen.queryByTestId("composer")).toBeNull();
  });

  it("cannot be started without CREATE_THREADS", async () => {
    await open(<ThreadScreen />, clubWith("SEND_IN_THREADS"));
    expect(await screen.findByText("You can read this thread but not reply in it.")).toBeOnTheScreen();
  });
});
