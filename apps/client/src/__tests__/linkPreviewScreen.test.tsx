import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";
import { Linking } from "react-native";

import ConversationScreen from "../../app/conversation/[id]";
import type { PreviewSource } from "../messaging/previewFetch";
import { previewSource } from "../platform/previewSource";
import { fakeAttachments } from "../testing/fakeAttachments";
import { messengersInDm, serverWith } from "../testing/messengers";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";

jest.mock("../platform/previewSource", () => ({ previewSource: jest.fn() }));
jest.setTimeout(60_000);

const html = new TextEncoder().encode(
  '<head><meta property="og:title" content="A post"><meta property="og:description" content="About things">' +
    '<meta property="og:image" content="https://blog.example/cover.png"></head>',
);

function source(viaInstance: boolean): PreviewSource {
  return {
    viaInstance,
    fetch: (url, kind) =>
      Promise.resolve(
        kind === "page"
          ? { contentType: "text/html", bytes: html }
          : url === "https://blog.example/cover.png"
            ? { contentType: "image/png", bytes: new Uint8Array([7, 7]) }
            : undefined,
      ),
  };
}

async function openConversation(viaInstance: boolean) {
  jest.mocked(previewSource).mockReturnValue(source(viaInstance));
  (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  const { ada: adaMessenger } = await messengersInDm();
  const store = fakeAttachments(serverWith([]).fetch);
  await renderWithApp(
    <ConversationScreen />,
    fakeConnection(serverWith([]), { user: ada, client: store.client }),
    fakeMessaging({ status: "ready", messenger: adaMessenger }),
  );
  await screen.findByText(/Messages here are end-to-end encrypted/);
  return { store, adaMessenger };
}

describe("link previews in a conversation", () => {
  it("sends the first link's preview with its image encrypted, and opens the link", async () => {
    const open = jest.spyOn(Linking, "openURL").mockResolvedValue(undefined);
    const { store, adaMessenger } = await openConversation(false);

    await fireEvent.changeText(screen.getByTestId("composer"), "read https://blog.example/post");
    expect(screen.queryByTestId("preview-disclosure")).toBeNull();
    await fireEvent.press(screen.getByTestId("composer-send"));

    await waitFor(async () => {
      expect((await adaMessenger.messages("dm")).length).toBe(1);
    });
    const [sent] = await adaMessenger.messages("dm");
    expect(sent?.payload).toMatchObject({
      preview: {
        url: "https://blog.example/post",
        title: "A post",
        description: "About things",
        image: { mimeType: "image/png" },
      },
    });
    expect(store.stored.size).toBe(1);
    const card = await screen.findByLabelText("Open A post");
    expect(screen.getByText("blog.example")).toBeOnTheScreen();
    await fireEvent.press(card);
    expect(open).toHaveBeenCalledWith("https://blog.example/post");
  });

  it("says when the instance sees the link, and hides previews that point elsewhere", async () => {
    const { adaMessenger } = await openConversation(true);
    await fireEvent.changeText(screen.getByTestId("composer"), "see https://blog.example/post");
    expect(
      screen.getByText("Previews of links go through your instance, which sees the link."),
    ).toBeOnTheScreen();
    await fireEvent.changeText(screen.getByTestId("composer"), "no link now");
    expect(screen.queryByTestId("preview-disclosure")).toBeNull();

    await act(async () => {
      await adaMessenger.send("dm", {
        kind: "text",
        text: "harmless https://blog.example",
        preview: { url: "https://elsewhere.example", title: "Trap" },
      });
      await adaMessenger.send("dm", {
        kind: "text",
        text: "bare https://bare.example",
        preview: { url: "https://bare.example" },
      });
    });
    expect(await screen.findByLabelText("Open bare.example")).toBeOnTheScreen();
    expect(screen.queryByLabelText("Open Trap")).toBeNull();
  });
});
