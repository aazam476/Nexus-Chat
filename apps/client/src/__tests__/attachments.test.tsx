import { act, fireEvent, screen, waitFor } from "@testing-library/react-native";

import ConversationScreen from "../../app/conversation/[id]";
import type { Connection } from "../connection/useConnectionState";
import type { PickedFile } from "../messaging/attachmentTransfer";
import { fileSize } from "../messaging/fileSize";
import { createI18n } from "../i18n/i18n";
import { pickDocument, pickMedia, saveReceivedFile } from "../platform/files";
import { fakeAttachments } from "../testing/fakeAttachments";
import { messengersInDm, serverWith } from "../testing/messengers";
import { ada, fakeConnection, fakeMessaging, renderWithApp } from "../testing/renderWithApp";

jest.mock("../platform/files", () => ({
  pickDocument: jest.fn(),
  pickMedia: jest.fn(),
  saveReceivedFile: jest.fn(),
}));
jest.setTimeout(60_000);

const notes: PickedFile = { name: "notes.txt", mimeType: "text/plain", bytes: new Uint8Array([104, 105]) };
const photo: PickedFile = {
  name: "photo.jpg",
  mimeType: "image/jpeg",
  bytes: new Uint8Array([1, 2, 3]),
  width: 640,
  height: 480,
  thumbnail: { bytes: new Uint8Array([9]), mimeType: "image/jpeg", width: 320, height: 240 },
};

function withLimit(connection: Connection, attachmentMaxBytes: number): Connection {
  const instance = connection.instance;
  if (instance === undefined) {
    throw new Error("the fake connection has an instance");
  }
  return {
    ...connection,
    instance: {
      ...instance,
      info: { ...instance.info, limits: { ...instance.info.limits, attachmentMaxBytes } },
    },
  };
}

async function openConversation(limit = 25 << 20) {
  (globalThis as { mockSearchParams?: object }).mockSearchParams = { id: "dm" };
  const { ada: adaMessenger } = await messengersInDm();
  const store = fakeAttachments(serverWith([]).fetch);
  const connection = withLimit(fakeConnection(serverWith([]), { user: ada, client: store.client }), limit);
  await renderWithApp(
    <ConversationScreen />,
    connection,
    fakeMessaging({ status: "ready", messenger: adaMessenger }),
  );
  await screen.findByText(/Messages here are end-to-end encrypted/);
  return { store, adaMessenger };
}

describe("attachments in a conversation", () => {
  it("sends encrypted files with a thumbnail and opens them", async () => {
    jest.mocked(pickDocument).mockResolvedValueOnce(notes).mockResolvedValueOnce(undefined);
    jest.mocked(pickMedia).mockResolvedValueOnce(photo);
    const { store, adaMessenger } = await openConversation();

    await fireEvent.press(screen.getByTestId("attach-file"));
    await fireEvent.press(screen.getByTestId("attach-file")); // cancelled
    await fireEvent.press(screen.getByTestId("attach-media"));
    await waitFor(() => {
      expect(screen.getByTestId("pending-1")).toBeOnTheScreen();
    });
    expect(screen.queryByTestId("pending-2")).toBeNull();
    await fireEvent.press(screen.getByTestId("pending-0"));
    expect(screen.getByLabelText("Remove photo.jpg")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("composer-send"));

    await waitFor(async () => {
      expect((await adaMessenger.messages("dm")).length).toBe(1);
    });
    const [sent] = await adaMessenger.messages("dm");
    const reference = sent?.payload.kind === "text" ? sent.payload.attachments?.[0] : undefined;
    expect(reference).toMatchObject({ name: "photo.jpg", width: 640, height: 480 });
    expect(store.stored.size).toBe(2);
    const preview = await screen.findByLabelText("Image: photo.jpg");
    expect(preview).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId(`attachment-${reference?.id ?? ""}`));
    await waitFor(() => {
      expect(saveReceivedFile).toHaveBeenCalledWith("photo.jpg", "image/jpeg", new Uint8Array([1, 2, 3]));
    });
  });

  it("refuses files over the limit and reports failed uploads and downloads", async () => {
    jest
      .mocked(pickDocument)
      .mockResolvedValueOnce(notes)
      .mockResolvedValueOnce({ ...notes, bytes: new Uint8Array([1]) });
    const { store, adaMessenger } = await openConversation(1);
    await fireEvent.press(screen.getByTestId("attach-file"));
    await waitFor(() => {
      expect(screen.getByTestId("pending-0")).toBeOnTheScreen();
    });
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("Files can be up to 1 byte on this instance.")).toBeOnTheScreen();

    await fireEvent.press(screen.getByTestId("pending-0"));
    await fireEvent.press(screen.getByTestId("attach-file"));
    await waitFor(() => {
      expect(screen.getByTestId("pending-0")).toBeOnTheScreen();
    });
    store.failNextUpload();
    await fireEvent.press(screen.getByTestId("composer-send"));
    expect(await screen.findByText("A file could not be sent. Try again.")).toBeOnTheScreen();
    await fireEvent.press(screen.getByTestId("composer-send"));
    await waitFor(async () => {
      expect((await adaMessenger.messages("dm")).length).toBe(1);
    });

    store.stored.clear();
    const [sent] = await adaMessenger.messages("dm");
    const id = sent?.payload.kind === "text" ? (sent.payload.attachments?.[0]?.id ?? "") : "";
    await act(async () => {
      await adaMessenger.sync("dm");
    });
    await fireEvent.press(await screen.findByTestId(`attachment-${id}`));
    expect(await screen.findByText("This file could not be opened.")).toBeOnTheScreen();
  });
});

describe("photo quality", () => {
  it("sends originals off mobile data unless a photo is switched to compressed, and says videos go as they are", async () => {
    const compressed = { bytes: new Uint8Array([7]), mimeType: "image/webp", width: 320, height: 240 };
    jest
      .mocked(pickMedia)
      .mockResolvedValueOnce({ ...photo, compressed })
      .mockResolvedValueOnce({ ...photo, name: "second.jpg", compressed })
      .mockResolvedValueOnce({ name: "clip.mp4", mimeType: "video/mp4", bytes: new Uint8Array([1]) });
    const { adaMessenger } = await openConversation();
    for (let picks = 0; picks < 3; picks++) {
      await fireEvent.press(screen.getByTestId("attach-media"));
    }
    expect(await screen.findByText("Videos are sent in original quality.")).toBeOnTheScreen();
    // Not on mobile data, so photos start as originals. Tapping the first twice
    // changes nothing; the second goes compressed.
    await fireEvent.press(screen.getByTestId("original-0"));
    await fireEvent.press(screen.getByTestId("original-0"));
    await fireEvent.press(screen.getByTestId("original-1"));
    await fireEvent.press(screen.getByTestId("composer-send"));

    await waitFor(async () => {
      expect((await adaMessenger.messages("dm")).length).toBe(1);
    });
    const [sent] = await adaMessenger.messages("dm");
    const files = sent?.payload.kind === "text" ? (sent.payload.attachments ?? []) : [];
    expect(files.map((file) => [file.name, file.mimeType, file.size])).toEqual([
      ["photo.jpg", "image/jpeg", 3],
      ["second.webp", "image/webp", 1],
      ["clip.mp4", "video/mp4", 1],
    ]);
  });
});

describe("file sizes", () => {
  it("reads naturally at every scale", () => {
    const { t } = createI18n();
    expect(fileSize(1, t)).toBe("1 byte");
    expect(fileSize(12, t)).toBe("12 bytes");
    expect(fileSize(2048, t)).toBe("2 KB");
    expect(fileSize(5.5 * 1024 ** 2, t)).toBe("5.5 MB");
    expect(fileSize(3 * 1024 ** 3, t)).toBe("3 GB");
  });
});
