import { getDocumentAsync } from "expo-document-picker";
import { File } from "expo-file-system";
import { ImageManipulator } from "expo-image-manipulator";
import { launchImageLibraryAsync } from "expo-image-picker";
import { shareAsync } from "expo-sharing";

import * as native from "../platform/files";
import * as web from "../platform/files.web";
import { describeImage } from "../platform/thumbnails";

jest.mock("expo-document-picker", () => ({ getDocumentAsync: jest.fn() }));
jest.mock("expo-image-picker", () => ({ launchImageLibraryAsync: jest.fn() }));
jest.mock("expo-sharing", () => ({ shareAsync: jest.fn() }));
jest.mock("expo-file-system", () => {
  class MockFile {
    static written: Uint8Array[] = [];
    readonly uri: string;
    constructor(...parts: unknown[]) {
      this.uri = parts.map(String).join("/");
    }
    bytes(): Promise<Uint8Array> {
      return Promise.resolve(new Uint8Array([4, 2]));
    }
    write(bytes: Uint8Array): Promise<void> {
      MockFile.written.push(bytes);
      return Promise.resolve();
    }
  }
  return { File: MockFile, Paths: { cache: "cache" } };
});
jest.mock("expo-image-manipulator", () => {
  const render = (width: number, height: number) => ({
    width,
    height,
    saveAsync: () => Promise.resolve({ uri: "thumb", width, height, base64: "CQ==" }),
  });
  return {
    SaveFormat: { JPEG: "jpeg", WEBP: "webp" },
    ImageManipulator: {
      manipulate: jest.fn(() => {
        let size = { width: 1280, height: 960 };
        const context = {
          resize: (to: { width: number }) => {
            size = { width: to.width, height: (to.width * 960) / 1280 };
            return context;
          },
          renderAsync: () => Promise.resolve(render(size.width, size.height)),
        };
        return context;
      }),
    },
  };
});

const document = (name: string, mimeType: string | undefined) => ({
  canceled: false,
  assets: [{ uri: "data:application/octet-stream;base64,BAI=", name, mimeType }],
});

describe("thumbnails", () => {
  it("reads the size and makes a JPEG at most 320 pixels wide", async () => {
    expect(await describeImage("photo")).toEqual({
      width: 1280,
      height: 960,
      thumbnail: { bytes: new Uint8Array([9]), mimeType: "image/jpeg", width: 320, height: 240 },
    });
  });

  it("gives nothing for something that is not an image", async () => {
    jest.spyOn(ImageManipulator, "manipulate").mockImplementationOnce(() => {
      throw new Error("not an image");
    });
    expect(await describeImage("text")).toEqual({});
  });
});

describe.each([
  ["native", native],
  ["web", web],
])("picking files (%s)", (_, files) => {
  beforeEach(() => {
    // The browser reads picked files through fetch; the test environment has none for data addresses.
    jest
      .spyOn(globalThis, "fetch")
      .mockImplementation(() => Promise.resolve(new Response(new Uint8Array([4, 2]))));
  });
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("reads a picked document, with image details for images", async () => {
    jest.mocked(getDocumentAsync).mockResolvedValueOnce(document("a.txt", "text/plain") as never);
    expect(await files.pickDocument()).toEqual({
      name: "a.txt",
      mimeType: "text/plain",
      bytes: new Uint8Array([4, 2]),
    });
    jest.mocked(getDocumentAsync).mockResolvedValueOnce(document("b.png", "image/png") as never);
    expect(await files.pickDocument()).toMatchObject({ name: "b.png", width: 1280, height: 960 });
    jest.mocked(getDocumentAsync).mockResolvedValueOnce(document("c", undefined) as never);
    expect(await files.pickDocument()).toMatchObject({ mimeType: "application/octet-stream" });
    jest.mocked(getDocumentAsync).mockResolvedValueOnce({ canceled: true, assets: null } as never);
    expect(await files.pickDocument()).toBeUndefined();
  });

  it("reads picked photos and videos", async () => {
    const asset = { uri: "data:image/jpeg;base64,BAI=", width: 1, height: 1 };
    jest
      .mocked(launchImageLibraryAsync)
      .mockResolvedValueOnce({ canceled: false, assets: [{ ...asset, type: "image" }] });
    expect(await files.pickMedia()).toMatchObject({ name: "photo", mimeType: "image/jpeg", width: 1280 });
    jest.mocked(launchImageLibraryAsync).mockResolvedValueOnce({
      canceled: false,
      assets: [{ ...asset, type: "video", fileName: "clip.mp4" }],
    });
    const video = await files.pickMedia();
    expect(video).toEqual({ name: "clip.mp4", mimeType: "video/mp4", bytes: new Uint8Array([4, 2]) });
    jest.mocked(launchImageLibraryAsync).mockResolvedValueOnce({ canceled: true, assets: null });
    expect(await files.pickMedia()).toBeUndefined();
  });
});

describe("saving received files", () => {
  it("shares the file on iOS and Android, under a safe name", async () => {
    await native.saveReceivedFile("a/b.txt", "text/plain", new Uint8Array([1]));
    expect((File as unknown as { written: Uint8Array[] }).written).toEqual([new Uint8Array([1])]);
    expect(shareAsync).toHaveBeenCalledWith("cache/a_b.txt", { mimeType: "text/plain" });
  });

  it("downloads it in the browser", async () => {
    jest.useFakeTimers();
    const click = jest.fn();
    const link = { href: "", download: "", click };
    const host = globalThis as { document?: unknown };
    host.document = { createElement: () => link };
    const revoke = jest.fn();
    URL.createObjectURL = jest.fn(() => "blob:x");
    URL.revokeObjectURL = revoke;
    await web.saveReceivedFile("b.txt", "text/plain", new Uint8Array([1]));
    expect(link).toMatchObject({ href: "blob:x", download: "b.txt" });
    expect(click).toHaveBeenCalled();
    jest.runAllTimers();
    expect(revoke).toHaveBeenCalledWith("blob:x");
    delete host.document;
    jest.useRealTimers();
  });
});
