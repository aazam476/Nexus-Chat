import {
  AttachmentTransferError,
  fetchAttachment,
  sendAttachment,
  type PickedFile,
} from "../messaging/attachmentTransfer";
import { fakeAttachments } from "../testing/fakeAttachments";

jest.setTimeout(60_000);

const bytes = (...values: number[]): Uint8Array<ArrayBuffer> => new Uint8Array(values);

describe("attachment transfer", () => {
  it("encrypts, uploads and fetches a file and its thumbnail", async () => {
    const store = fakeAttachments();
    const file: PickedFile = {
      name: "photo.png",
      mimeType: "image/png",
      bytes: bytes(1, 2, 3, 4),
      width: 640,
      height: 480,
      thumbnail: { bytes: bytes(9, 9), mimeType: "image/jpeg", width: 320, height: 240 },
    };
    const reference = await sendAttachment(
      { client: store.client, conversationId: "dm", maxBytes: 100 },
      file,
    );

    expect(reference).toMatchObject({
      name: "photo.png",
      mimeType: "image/png",
      size: 4,
      width: 640,
      height: 480,
      scheme: "aes-256-gcm",
    });
    expect(reference.thumbnail).toMatchObject({ mimeType: "image/jpeg", width: 320, height: 240 });
    // The server only ever sees ciphertext.
    for (const stored of store.stored.values()) {
      expect([...stored]).not.toEqual([1, 2, 3, 4]);
    }
    expect([...(await fetchAttachment(store.client, "dm", reference))]).toEqual([1, 2, 3, 4]);
    expect([...(await fetchAttachment(store.client, "dm", reference.thumbnail ?? reference))]).toEqual([
      9, 9,
    ]);
  });

  it("streams files over 64 MiB", async () => {
    const store = fakeAttachments();
    const large = new Uint8Array(64 * 1024 * 1024 + 5).fill(7);
    const reference = await sendAttachment(
      { client: store.client, conversationId: "dm", maxBytes: large.length },
      { name: "big.bin", mimeType: "application/octet-stream", bytes: large },
    );

    expect(reference.scheme).toBe("xchacha20poly1305-secretstream");
    expect(reference.nonce).toBeUndefined();
    const back = await fetchAttachment(store.client, "dm", reference);
    expect(back.length).toBe(large.length);
    expect(back[back.length - 1]).toBe(7);
  });

  it("refuses files over the limit, failed uploads and missing files", async () => {
    const store = fakeAttachments();
    const file: PickedFile = { name: "a.txt", mimeType: "text/plain", bytes: bytes(1, 2, 3) };
    await expect(
      sendAttachment({ client: store.client, conversationId: "dm", maxBytes: 2 }, file),
    ).rejects.toEqual(new AttachmentTransferError("too_large"));
    store.failNextUpload();
    await expect(
      sendAttachment({ client: store.client, conversationId: "dm", maxBytes: 10 }, file),
    ).rejects.toEqual(new AttachmentTransferError("upload_failed"));
    const reference = await sendAttachment(
      { client: store.client, conversationId: "dm", maxBytes: 10 },
      file,
    );
    await expect(fetchAttachment(store.client, "dm", { ...reference, id: "missing" })).rejects.toEqual(
      new AttachmentTransferError("download_failed"),
    );
    await expect(
      fetchAttachment(store.client, "dm", {
        ...reference,
        key: reference.key.replace(/^./, reference.key.startsWith("A") ? "B" : "A"),
      }),
    ).rejects.toThrow();
  });
});
