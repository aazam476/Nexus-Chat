import { fireEvent, screen } from "@testing-library/react-native";
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
