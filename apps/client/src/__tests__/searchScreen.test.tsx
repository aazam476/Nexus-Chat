import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import SearchScreen from "../../app/search";
import ConversationScreen from "../../app/conversation/[id]";
import { goodTimeToBackfill } from "../platform/backfillConditions";
import { filtersOf, senderOf } from "../search/request";
import type { SearchRequest } from "../screens/search/SearchForm";
import { dm, messengersInDm, renderAs, serverWith } from "../testing/messengers";
import { answerServer, audienceId, general, messengersInServer, serverId } from "../testing/servers";
import { audiencesOf } from "../servers/audiences";
import { bookClub } from "../testing/servers";

jest.setTimeout(60_000);
jest.mock("../platform/backfillConditions", () => ({ goodTimeToBackfill: jest.fn() }));

const params = (values: object): void => {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = values;
};

/** Waits out the pause before a search runs, then for its results. */
async function type(testID: string, text: string): Promise<void> {
  await fireEvent.changeText(screen.getByTestId(testID), text);
}

describe("searching messages", () => {
  beforeEach(() => {
    params({});
  });

  it("finds messages on this device as the user types, and opens them", async () => {
    const { ada, grace } = await messengersInDm();
    await grace.send("dm", { kind: "text", text: "The quarterly **report** is ready" });
    await ada.sync("dm");
    await ada.send("dm", { kind: "text", text: "Thanks for the report" });
    await renderAs(<SearchScreen />, ada, serverWith([dm]));
    expect(screen.getByText("Type some words, or choose a filter.")).toBeOnTheScreen();

    await type("search-query", "rep");
    expect(await screen.findByText("2 messages")).toBeOnTheScreen();
    expect(screen.getByText("The quarterly report is ready")).toBeOnTheScreen();

    await type("search-from", "grace");
    expect(await screen.findByText("1 message")).toBeOnTheScreen();
    await fireEvent.press(screen.getByText("The quarterly report is ready"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/conversation/[id]", params: { id: "dm" } });

    await fireEvent.press(screen.getByTestId("search-when-1"));
    expect(await screen.findByText("1 message")).toBeOnTheScreen();
    await type("search-query", "nothing like this");
    expect(await screen.findByText("No messages match.")).toBeOnTheScreen();
  });

  it("opens from a conversation, limited to it", async () => {
    const { ada } = await messengersInDm();
    params({ id: "dm" });
    await renderAs(<ConversationScreen />, ada, serverWith([dm]));
    await fireEvent.press(await screen.findByTestId("search-here"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/search", params: { in: "dm" } });
  });

  it("searches only where it was opened from, until widened", async () => {
    const { ada } = await messengersInServer();
    const audience = audiencesOf(bookClub(), [])[0];
    if (audience === undefined) {
      throw new Error("no audience");
    }
    await ada.sendInChannel(audience, general, { kind: "text", text: "Chapter one tonight" });
    const server = serverWith([dm]);
    answerServer(server);
    params({ in: general });
    await renderAs(<SearchScreen />, ada, server);
    await type("search-query", "chapter");
    expect(await screen.findByText("1 message")).toBeOnTheScreen();
    expect(await screen.findByText(/^#⁨general⁩ · ⁨Book club⁩ · /)).toBeOnTheScreen();
    await fireEvent.press(screen.getByText("Chapter one tonight"));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]",
      params: { id: serverId, channelId: general },
    });
    expect(audienceId).toBeDefined();

    // Older messages wait for a good time on a phone, unless the user says go ahead.
    jest.mocked(goodTimeToBackfill).mockResolvedValue(false);
    expect(screen.getByText("Messages older than 90 days are not searched here yet.")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("search-older"));
    expect(await screen.findByText(/^1 message indexed so far/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("search-older-now"));
    expect(await screen.findByText("All of the history here is searchable: 1 message.")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("search-where-1"));
    await fireEvent.press(screen.getByTestId("search-attachments-1"));
    expect(await screen.findByText("No messages match.")).toBeOnTheScreen();
  });
});

describe("finding names on the server", () => {
  beforeEach(() => {
    params({});
  });

  const found = {
    servers: [{ id: serverId, name: "Book club" }],
    channels: [
      { id: general, serverId, serverName: "Book club", kind: "text", name: "general" },
      { id: "voice-1", serverId, serverName: "Book club", kind: "voice", name: "General" },
    ],
    people: [{ id: "user-grace", username: "grace", displayName: "Grace Hopper" }],
  };

  it("asks the server only once the user says so, and opens what it finds", async () => {
    const { ada } = await messengersInDm();
    const server = serverWith([dm]);
    server.answer("GET /api/v1/search", { status: 200, body: found });
    server.answer("POST /api/v1/conversations/dms", { status: 200, body: dm });
    await renderAs(<SearchScreen />, ada, server);
    await type("search-query", "gen");
    expect(await screen.findByText("No messages match.")).toBeOnTheScreen();
    expect(server.requests.filter((request) => request.path === "/api/v1/search")).toEqual([]);

    await fireEvent.press(screen.getByTestId("search-names-start"));
    expect(await screen.findByText("Servers, channels and people")).toBeOnTheScreen();
    expect(server.requests.find((request) => request.path === "/api/v1/search")?.search).toBe("?q=gen");
    expect(screen.getByText("Voice channel in \u2068Book club\u2069")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId(`search-channel-${general}`));
    expect(router.push).toHaveBeenCalledWith({
      pathname: "/server/[id]/channel/[channelId]",
      params: { id: serverId, channelId: general },
    });
    await fireEvent.press(screen.getByTestId("search-channel-voice-1"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/server/[id]", params: { id: serverId } });
    await fireEvent.press(screen.getByTestId(`search-server-${serverId}`));
    await fireEvent.press(screen.getByTestId("search-person-user-grace"));
    await waitFor(() => {
      expect(router.push).toHaveBeenCalledWith({ pathname: "/conversation/[id]", params: { id: "dm" } });
    });

    await type("search-query", "g");
    expect(await screen.findByText(/^Your server searches names only/)).toBeOnTheScreen();
  });

  it("says when nothing matches or a person cannot be messaged", async () => {
    const { ada } = await messengersInDm();
    const server = serverWith([dm]);
    server.answer("GET /api/v1/search", { status: 200, body: { ...found, servers: [], channels: [] } });
    server.answer("POST /api/v1/conversations/dms", {
      status: 403,
      body: { code: "forbidden", message: "no" },
    });
    await renderAs(<SearchScreen />, ada, server);
    await fireEvent.press(screen.getByTestId("search-names-start"));
    await type("search-query", "grace");
    await fireEvent.press(await screen.findByTestId("search-person-user-grace"));
    expect(await screen.findByText(/^A direct message could not be opened/)).toBeOnTheScreen();

    server.answer("GET /api/v1/search", { status: 200, body: { servers: [], channels: [], people: [] } });
    await type("search-query", "zzz");
    expect(await screen.findByText("No servers, channels or people have names like that.")).toBeOnTheScreen();
  });

  it("is not offered when searching one conversation", async () => {
    const { ada } = await messengersInDm();
    params({ in: "dm" });
    await renderAs(<SearchScreen />, ada);
    expect(screen.queryByTestId("search-names-start")).toBeNull();
  });
});

describe("search requests", () => {
  const request: SearchRequest = { query: "", from: "", attachments: false, period: "any", here: false };
  const now = new Date("2026-11-08T12:00:00Z");

  it("name senders as the index does", () => {
    expect(senderOf("  Grace ", "chat.example.com")).toBe("@grace@chat.example.com");
    expect(senderOf("@sam@other.example", "chat.example.com")).toBe("@sam@other.example");
    expect(senderOf("", "chat.example.com")).toBeUndefined();
  });

  it("turn into index filters", () => {
    const context = { scope: "dm", domain: "chat.example.com", now };
    expect(filtersOf(request, context)).toEqual({});
    expect(
      filtersOf({ ...request, here: true, attachments: true, period: "week", from: "ada" }, context),
    ).toEqual({
      from: "@ada@chat.example.com",
      in: ["dm"],
      hasAttachments: true,
      after: "2026-11-01T12:00:00.000Z",
    });
    expect(filtersOf({ ...request, here: true }, { ...context, scope: undefined })).toEqual({});
  });
});
