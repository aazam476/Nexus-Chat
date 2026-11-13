import { act, fireEvent, screen } from "@testing-library/react-native";

import ConversationScreen from "../../app/conversation/[id]";
import Conversations from "../../app/conversations";
import { isolate } from "../i18n/bidi";
import { insertMention, mentionQuery, mentionsUser, suggestMentions } from "../messaging/mentions";
import { dm, grace, messengersInDm, renderAs, serverWith } from "../testing/messengers";
import { ada } from "../testing/renderWithApp";

jest.setTimeout(60_000);

const people = [
  { id: "a", username: "ada", displayName: "Ada" },
  { id: "g", username: "grace", displayName: "Grace Hopper" },
  { id: "h", username: "hopper2", displayName: "Hopper" },
];

describe("mentions while typing", () => {
  it("finds the name after a trailing @", () => {
    expect(mentionQuery("hi @gra")).toBe("gra");
    expect(mentionQuery("@")).toBe("");
    expect(mentionQuery("mail me@example")).toBeUndefined();
    expect(mentionQuery("@grace done")).toBeUndefined();
  });

  it("suggests other members by username or display name, at most five", () => {
    expect(suggestMentions("ho", people, "a").map((person) => person.id)).toEqual(["h"]);
    expect(suggestMentions("GR", people, "a").map((person) => person.id)).toEqual(["g"]);
    expect(suggestMentions("ad", people, "a")).toEqual([]);
    const many = Array.from({ length: 8 }, (_, index) => ({
      id: String(index),
      username: `u${String(index)}`,
      displayName: "U",
    }));
    expect(suggestMentions("", many, "x")).toHaveLength(5);
  });

  it("writes the chosen mention in place of what was typed", () => {
    expect(insertMention("hi @gra", { id: "g", username: "grace", displayName: "Grace" })).toBe("hi <@g> ");
  });

  it("knows which messages mention someone", () => {
    expect(mentionsUser("hey <@g>", "g")).toBe(true);
    expect(mentionsUser("**look <@g>**", "g")).toBe(true);
    expect(mentionsUser("hey <@h> and <#c> and <@&r>", "g")).toBe(false);
    expect(mentionsUser("@everyone stand up", "g")).toBe(true);
    expect(mentionsUser("```\n<@g>\n```", "g")).toBe(false);
  });
});

describe("mentions in a conversation", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  });

  it("suggests members, sends the mention and shows it by name", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await renderAs(<ConversationScreen />, adaMessenger);
    await screen.findByText(/Messages here are end-to-end encrypted/);

    await fireEvent.changeText(screen.getByTestId("composer"), "thanks @gr");
    await fireEvent.press(await screen.findByTestId("mention-grace"));
    expect(screen.getByTestId("composer").props.value).toBe(`thanks <@${grace.id}> `);
    expect(screen.queryByTestId("mention-suggestions")).toBeNull();
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText(`@${isolate("Grace")}`)).toBeOnTheScreen();

    await act(async () => {
      await graceMessenger.send("dm", { kind: "text", text: `you too <@${ada.id}> and <@unknown-person>` });
      await adaMessenger.sync("dm");
    });
    expect(await screen.findByText(`@${isolate("Ada")}`)).toBeOnTheScreen();
    expect(screen.getByText("@someone")).toBeOnTheScreen();
  });

  it("counts unread mentions of the user in the list", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await graceMessenger.send("dm", { kind: "text", text: `ping <@${ada.id}>` });
    await graceMessenger.send("dm", { kind: "text", text: "@here lunch?" });
    await graceMessenger.send("dm", { kind: "text", text: "no mention" });
    await adaMessenger.send("dm", { kind: "text", text: `my own <@${ada.id}>` });
    await adaMessenger.sync("dm");
    const server = serverWith([dm]);
    server.answer("GET /api/v1/read-state", {
      status: 200,
      body: [{ conversationId: "dm", lastReadId: 0, unreadCount: 3 }],
    });
    await renderAs(<Conversations />, adaMessenger, server);
    expect(await screen.findByLabelText("2 unread mentions of you")).toHaveTextContent("@2");
  });
});
