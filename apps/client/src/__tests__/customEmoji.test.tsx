import type { ServerDetails, ServerEmoji } from "@nexus-chat/api-client";
import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import ChannelScreen from "../../app/server/[id]/channel/[channelId]";
import EmojiSettings from "../../app/server/[id]/emoji";
import ServerSettings from "../../app/server/[id]/settings";
import { pickOriginalImage } from "../platform/pickImage";
import { emojiSizes } from "../screens/conversation/CustomEmoji";
import { fakeServer, type FakeServer } from "../testing/fakeServer";
import { fakeSockets, nth } from "../testing/fakeSocket";
import { ada as adaUser, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";
import { renderAs } from "../testing/messengers";
import { answerServer, bookClub, general, messengersInServer, serverId } from "../testing/servers";

jest.setTimeout(60_000);
jest.mock("../platform/pickImage", () => ({ pickOriginalImage: jest.fn(), pickSquareImage: jest.fn() }));

const base = `/api/v1/servers/${serverId}`;
const emoji = (name: string, index: number): ServerEmoji => {
  const id = `0195f0e0-0000-7000-8000-00000000e0${String(10 + index)}`;
  return { id, name, animated: false, url: `/api/v1/emoji/${id}`, createdAt: "2026-11-01T00:00:00Z" };
};
const wave = emoji("wave", 0);
const names = ["wave", "party", "cat", "dog", "fox"];
const many = names.map(emoji);

/** Book club where Ada manages emoji and may use them in #general. */
function emojiClub(): ServerDetails {
  const details = bookClub();
  return {
    ...details,
    permissions: [...details.permissions, "MANAGE_EMOJI", "USE_CUSTOM_EMOJI"],
    channels: details.channels.map((channel) =>
      channel.id === general
        ? { ...channel, permissions: [...channel.permissions, "USE_CUSTOM_EMOJI"] }
        : channel,
    ),
  };
}

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

async function open(ui: React.ReactElement, list: ServerEmoji[], details = emojiClub()): Promise<FakeServer> {
  const { ada } = await messengersInServer();
  const server = fakeServer();
  answerServer(server, details);
  server.answer(`GET ${base}/emoji`, { status: 200, body: list });
  server.answer(`GET ${base}/read-state`, { status: 200, body: [] });
  await renderAs(ui, ada, server);
  return server;
}

describe("the emoji settings", () => {
  beforeEach(() => {
    params({ id: serverId });
  });

  it("list the server's emoji, add one from a picture, and remove one", async () => {
    const server = await open(<EmojiSettings />, [wave]);
    expect(await screen.findByText(":wave:")).toBeOnTheScreen();
    expect(screen.getByText("1 of 100")).toBeOnTheScreen();
    expect(screen.getByTestId(`emoji-${wave.id}`).props.style).toMatchObject({ width: emojiSizes.alone });

    server.answer(`POST ${base}/emoji`, { status: 201, body: { id: "new" } });
    jest
      .mocked(pickOriginalImage)
      .mockResolvedValueOnce(undefined)
      .mockResolvedValueOnce(new Blob(["gif"]));
    expect(screen.getByTestId("emoji-add")).toBeDisabled();
    await fireEvent.changeText(screen.getByTestId("emoji-name"), "Party_Parrot");
    await fireEvent.press(screen.getByTestId("emoji-add"));
    expect(server.requests.some((request) => request.method === "POST")).toBe(false);
    await fireEvent.press(screen.getByTestId("emoji-add"));
    await waitFor(() => {
      expect(server.requests.find((request) => request.method === "POST")?.search).toBe("?name=party_parrot");
    });
    await waitFor(() => {
      expect(screen.getByTestId("emoji-name").props.value).toBe("");
    });

    server.answer(`DELETE ${base}/emoji/${wave.id}`, { status: 204 });
    await fireEvent.press(screen.getByRole("button", { name: "Remove :wave:" }));
    await fireEvent.press(screen.getByTestId("emoji-remove-wave-confirm"));
    await waitFor(() => {
      expect(server.requests.some((request) => request.method === "DELETE")).toBe(true);
    });
  });

  it("explain why an emoji was refused, and say when there are none", async () => {
    const server = await open(<EmojiSettings />, []);
    expect(await screen.findByText("No emoji yet.")).toBeOnTheScreen();
    server.answer(`POST ${base}/emoji`, {
      status: 409,
      body: { code: "emoji_name_taken", message: "taken" },
    });
    jest.mocked(pickOriginalImage).mockResolvedValueOnce(new Blob(["png"]));
    await fireEvent.changeText(screen.getByTestId("emoji-name"), "wave");
    await fireEvent.press(screen.getByTestId("emoji-add"));
    expect(await screen.findByText("This server already has an emoji with that name.")).toBeOnTheScreen();
  });

  it("reload when the gateway says the server changed", async () => {
    const sockets = fakeSockets();
    const { ada } = await messengersInServer();
    const server = fakeServer();
    answerServer(server, emojiClub());
    server.answer(`GET ${base}/emoji`, { status: 200, body: [] });
    await renderWithApp(
      <EmojiSettings />,
      fakeConnection(server, { user: adaUser, sessionToken: "nxs_t" }),
      fakeMessaging({ status: "ready", messenger: ada }),
    );
    expect(await screen.findByText("No emoji yet.")).toBeOnTheScreen();
    server.answer(`GET ${base}/emoji`, { status: 200, body: [wave] });
    await act(async () => {
      const socket = nth(sockets, 0);
      socket.receive({ type: "server_update", seq: 1, data: { serverId: "another" } });
      socket.receive({ type: "server_update", seq: 2, data: { serverId } });
      await Promise.resolve();
    });
    expect(await screen.findByText(":wave:")).toBeOnTheScreen();
  });

  it("are offered in the server settings to those who manage emoji", async () => {
    await open(<ServerSettings />, []);
    expect(await screen.findByTestId("manage-emoji")).toBeOnTheScreen();
  });
});

describe("custom emoji in a channel", () => {
  beforeEach(() => {
    params({ id: serverId, channelId: general });
  });

  it("are suggested while typing, sent, and drawn in messages", async () => {
    await open(<ChannelScreen />, many);
    await screen.findByTestId("composer");
    await fireEvent.changeText(screen.getByTestId("composer"), "hello :wa");
    await fireEvent.press(await screen.findByTestId("emoji-suggestion-wave"));
    expect(screen.getByTestId("composer").props.value).toBe(`hello <:wave:${wave.id}> `);
    await fireEvent.press(screen.getByTestId("composer-send"));
    const drawn = await screen.findByLabelText(":wave:");
    expect(drawn.props.source).toEqual({ uri: `https://chat.example.com/api/v1/emoji/${wave.id}` });
    expect(drawn.props.style).toMatchObject({ width: emojiSizes.inline });

    // A picture that cannot load is replaced by the emoji's name.
    await act(async () => {
      (drawn.props as { onError: () => void }).onError();
      await Promise.resolve();
    });
    expect(screen.getByText(":wave:")).toBeOnTheScreen();
  });

  it("are drawn large in a message of only emoji, and react to messages", async () => {
    await open(<ChannelScreen />, many);
    await screen.findByTestId("composer");
    await fireEvent.changeText(screen.getByTestId("composer"), `<:wave:${wave.id}>`);
    await fireEvent.press(screen.getByTestId("composer-send"));
    const drawn = await screen.findByLabelText(":wave:");
    expect(drawn.props.style).toMatchObject({ width: emojiSizes.alone });

    const [message] = screen.getAllByTestId(/^message-[0-9a-f-]{36}$/);
    if (message === undefined) {
      throw new Error("the message is not shown");
    }
    await fireEvent.press(message);
    expect(screen.queryByTestId("react-fox")).toBeNull();
    await fireEvent.press(screen.getByTestId("more-emoji"));
    await fireEvent.press(screen.getByLabelText("Fewer emoji"));
    expect(screen.queryByTestId("react-fox")).toBeNull();
    await fireEvent.press(screen.getByLabelText("More emoji"));
    await fireEvent.press(screen.getByTestId("react-fox"));
    expect(await screen.findByLabelText(/^:fox: 1, from/)).toBeOnTheScreen();
  });

  it("are not offered to those who may not use them there", async () => {
    const details = emojiClub();
    const channels = details.channels.map((channel) =>
      channel.id === general
        ? { ...channel, permissions: channel.permissions.filter((name) => name !== "USE_CUSTOM_EMOJI") }
        : channel,
    );
    await open(<ChannelScreen />, many, { ...details, channels });
    await screen.findByTestId("composer");
    await fireEvent.changeText(screen.getByTestId("composer"), "hello :wa");
    expect(screen.queryByTestId("emoji-suggestions")).toBeNull();
  });
});
