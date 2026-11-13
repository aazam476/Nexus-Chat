import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { router } from "expo-router";

import ConversationScreen from "../../app/conversation/[id]";
import Conversations from "../../app/conversations";
import { isolate } from "../i18n/bidi";
import { createI18n } from "../i18n/i18n";
import { lastLine } from "../screens/conversation/lastLine";
import { MessageBody } from "../screens/conversation/MessageBody";
import { dm, grace, messengersInDm, renderAs, serverWith } from "../testing/messengers";
import { fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";

jest.setTimeout(60_000);

describe("the conversation list", () => {
  it("names each conversation by the other person and shows the last message", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    await graceMessenger.send("dm", { kind: "text", text: "see you **soon**" });
    await adaMessenger.sync("dm");
    const group = {
      ...dm,
      id: "group",
      kind: "group",
      members: [...dm.members, { id: "x", username: "x", displayName: "Xavier" }],
    };
    const named = { ...group, id: "named", name: "Book club" };
    await renderAs(<Conversations />, adaMessenger, serverWith([dm, group, named]));

    expect(await screen.findByText(isolate("Grace"))).toBeOnTheScreen();
    expect(await screen.findByText("see you soon")).toBeOnTheScreen();
    expect(screen.getByText(`${isolate("Grace")} and 1 other`)).toBeOnTheScreen();
    expect(screen.getByText(isolate("Book club"))).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("conversation-dm"));
    expect(router.push).toHaveBeenCalledWith({ pathname: "/conversation/[id]", params: { id: "dm" } });
  });

  it("says when there are no conversations yet", async () => {
    const { ada: adaMessenger } = await messengersInDm();
    await renderAs(<Conversations />, adaMessenger, serverWith([]));
    expect(await screen.findByText(/No conversations yet/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("conversations-back"));
    expect(router.back).toHaveBeenCalled();
  });
});

describe("a conversation", () => {
  beforeEach(() => {
    (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  });

  it("shows messages and sends, replies, edits, reacts and deletes", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    const greeting = await graceMessenger.send("dm", { kind: "text", text: "hello **Ada**" });
    await adaMessenger.sync("dm");
    await renderAs(<ConversationScreen />, adaMessenger);
    expect(await screen.findByText("Ada")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId(`message-${greeting}`));
    await fireEvent.press(screen.getByText("Reply"));
    expect(screen.getByText(`Replying to ${isolate("Grace")}`)).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("composer"), "hi Grace");
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("hi Grace")).toBeOnTheScreen();
    expect(screen.getByText(`Reply to ${isolate("Grace")}: hello **Ada**`)).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId(`message-${greeting}`));
    await fireEvent.press(screen.getByLabelText("React with 👍"));
    expect(await screen.findByText("👍 1")).toBeOnTheScreen();
    await fireEvent.press(screen.getByText("👍 1"));
    await waitFor(() => {
      expect(screen.queryByText("👍 1")).toBeNull();
    });

    const own = (await adaMessenger.messages("dm")).find(
      (message) => message.payload.kind === "text" && message.payload.text === "hi Grace",
    );
    await fireEvent.press(screen.getByTestId(`message-${own?.id ?? ""}`));
    await fireEvent.press(screen.getByText("Edit"));
    expect(screen.getByText("Editing your message")).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("composer"), "hi Grace!");
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("(edited)")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId(`message-${own?.id ?? ""}`));
    await fireEvent.press(screen.getByTestId("message-delete"));
    expect(await screen.findByText("This message was deleted.")).toBeOnTheScreen();
  });

  it("cancels a reply, refuses an over-long message and says when a send fails", async () => {
    const { ada: adaMessenger, grace: graceMessenger } = await messengersInDm();
    const greeting = await graceMessenger.send("dm", { kind: "text", text: "hey" });
    await adaMessenger.sync("dm");
    await renderAs(<ConversationScreen />, adaMessenger);
    await screen.findByText("hey");

    await fireEvent.press(screen.getByTestId(`message-${greeting}`));
    await fireEvent.press(screen.getByText("Reply"));
    await fireEvent.press(screen.getByText("Cancel"));
    expect(screen.queryByText(/Replying to/)).toBeNull();
    await fireEvent.changeText(screen.getByTestId("composer"), "x".repeat(4001));
    expect(screen.getByText("Messages can be up to 4000 characters.")).toBeOnTheScreen();

    jest.spyOn(adaMessenger, "send").mockRejectedValueOnce(new Error("offline"));
    await fireEvent.changeText(screen.getByTestId("composer"), "later");
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("The message could not be sent. Try again.")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("conversation-back"));
    expect(router.back).toHaveBeenCalled();
  });

  it("is empty until the first message, and warns when an identity changed", async () => {
    const { ada: adaMessenger } = await messengersInDm();
    jest.spyOn(adaMessenger, "identityChanges").mockReturnValue([grace.id]);
    const accept = jest.spyOn(adaMessenger, "acceptIdentityChange").mockResolvedValue(undefined);
    await renderAs(<ConversationScreen />, adaMessenger);
    expect(await screen.findByText(/Messages here are end-to-end encrypted/)).toBeOnTheScreen();
    expect(await screen.findByText(/encryption identity of/)).toBeOnTheScreen();
    await fireEvent.press(screen.getByText("Trust the new identity"));
    expect(accept).toHaveBeenCalledWith(grace.id);
  });
});

describe("a message's Markdown", () => {
  it("draws every kind of formatting", async () => {
    await renderWithApp(
      <MessageBody
        text={
          "**b** *i* ~~s~~ `c` ||secret|| [site](https://example.com) <@u1> <#c1> <@&r1> @here\n> quoted\n```\ncode\n```"
        }
      />,
    );
    expect(screen.getByText("code")).toBeOnTheScreen();
    expect(screen.getByText("quoted")).toBeOnTheScreen();
    // Outside a conversation nobody is known by ID; channels and roles show their ID until M5.
    expect(screen.getByText("@someone")).toBeOnTheScreen();
    expect(screen.getByText("#c1")).toBeOnTheScreen();
    expect(screen.getByText("@r1")).toBeOnTheScreen();
    expect(screen.getByText("@here")).toBeOnTheScreen();
    // Hidden, the spoiler's text is not on the screen at all.
    expect(screen.queryByText(/secret/)).toBeNull();
    await fireEvent.press(screen.getByRole("button", { name: "Spoiler" }));
    expect(screen.getByText(/secret/)).toBeOnTheScreen();
    expect(screen.queryByRole("button", { name: "Spoiler" })).toBeNull();
    await fireEvent.press(screen.getByText("site"));
  });
});

describe("a message as one line", () => {
  const t = createI18n().t;
  const item = {
    id: "m",
    sender: "@ada@chat.example.com",
    sentAt: "",
    text: "",
    edited: false,
    deleted: false,
    attachments: [],
    reactions: [],
  };
  const attachment = {
    id: "a",
    name: "photo.png",
    mimeType: "image/png",
    size: 1,
    scheme: "aes-256-gcm",
    key: "",
    ciphertextSha256: "",
  };

  it("says what a deleted or attachment-only message was", () => {
    expect(lastLine({ ...item, deleted: true }, t)).toBe("This message was deleted.");
    expect(lastLine({ ...item, attachments: [attachment] }, t)).toBe("Attachment: photo.png");
    expect(lastLine(item, t)).toBe("");
    expect(lastLine({ ...item, text: "two\n\nlines" }, t)).toBe("two lines");
    const long = lastLine({ ...item, text: "🎉".repeat(150) }, t);
    expect(Array.from(long)).toHaveLength(100);
    expect(long.endsWith("…")).toBe(true);
  });

  it("sends a logged-out user away from the list", async () => {
    await renderWithApp(<Conversations />, fakeConnection(serverWith([])), fakeMessaging({ status: "off" }));
    expect(router.replace).toHaveBeenCalledWith("/");
  });
});
