import { ImageManipulator } from "expo-image-manipulator";
import { getNetworkStateAsync, NetworkStateType } from "expo-network";

import type { PickedFile } from "../messaging/attachmentTransfer";
import { versionToSend } from "../messaging/compression";
import { onMobileData } from "../platform/network";
import { onMobileData as onMobileDataOnWeb } from "../platform/network.web";
import { compressedSize, prepareImage } from "../platform/thumbnails";
import { compressByDefault, readMediaQuality } from "../settings/mediaQuality";

const mockImage = { width: 1280, height: 960, saved: [] as { format: string; compress: number }[] };

jest.mock("expo-network", () => ({
  NetworkStateType: { CELLULAR: "CELLULAR", WIFI: "WIFI" },
  getNetworkStateAsync: jest.fn(),
}));
jest.mock("expo-image-manipulator", () => ({
  SaveFormat: { JPEG: "jpeg", WEBP: "webp" },
  ImageManipulator: {
    manipulate: jest.fn(() => {
      let size = { width: mockImage.width, height: mockImage.height };
      const context = {
        resize: (to: { width: number }) => {
          size = { width: to.width, height: Math.round((to.width * mockImage.height) / mockImage.width) };
          return context;
        },
        renderAsync: () =>
          Promise.resolve({
            ...size,
            saveAsync: (options: { format: string; compress: number }) => {
              mockImage.saved.push(options);
              return Promise.resolve({ ...size, uri: "out", base64: "AQID" });
            },
          }),
      };
      return context;
    }),
  },
}));

const photo = (mimeType: string, bytes = new Uint8Array(10)): PickedFile => ({
  name: "IMG_1.jpeg",
  mimeType,
  bytes,
});

describe("preparing a photo to send", () => {
  beforeEach(() => {
    mockImage.saved = [];
    mockImage.width = 1280;
    mockImage.height = 960;
  });

  it("makes a WebP copy at quality 82 alongside the original", async () => {
    const prepared = await prepareImage("photo", photo("image/jpeg"));
    expect(prepared.compressed).toEqual({
      bytes: new Uint8Array([1, 2, 3]),
      mimeType: "image/webp",
      width: 1280,
      height: 960,
    });
    expect(mockImage.saved).toContainEqual(expect.objectContaining({ format: "webp", compress: 0.82 }));
    expect(prepared.bytes).toHaveLength(10);
  });

  it("shrinks very large photos to 2560 pixels on the longest side", async () => {
    mockImage.width = 8000;
    mockImage.height = 6000;
    const prepared = await prepareImage("big", photo("image/jpeg"));
    expect(prepared.compressed?.width).toBe(compressedSize);
    expect(prepared.compressed?.height).toBe(1920);
  });

  it("re-encodes HEIC originals as JPEG, and leaves transparent images uncompressed", async () => {
    const heic = await prepareImage("heic", photo("image/heic"));
    expect(heic).toMatchObject({
      name: "IMG_1.jpg",
      mimeType: "image/jpeg",
      bytes: new Uint8Array([1, 2, 3]),
    });
    const transparent = new Uint8Array(30);
    transparent[25] = 6;
    expect((await prepareImage("png", photo("image/png", transparent))).compressed).toBeUndefined();
  });

  it("sends what it can when the image cannot be re-encoded", async () => {
    const manipulate = jest.spyOn(ImageManipulator, "manipulate").mockImplementation(() => {
      throw new Error("unreadable");
    });
    const prepared = await prepareImage("broken", photo("image/heic"));
    expect(prepared).toEqual(photo("image/heic"));
    manipulate.mockRestore();
  });
});

describe("choosing what goes out", () => {
  const compressed = { bytes: new Uint8Array(4), mimeType: "image/webp", width: 2, height: 1 };

  it("sends the smaller compressed copy unless the original is chosen", () => {
    const file = { ...photo("image/jpeg"), compressed, width: 4, height: 2 };
    expect(versionToSend(file)).toEqual({
      name: "IMG_1.webp",
      mimeType: "image/webp",
      bytes: compressed.bytes,
      width: 2,
      height: 1,
    });
    expect(versionToSend({ ...file, sendOriginal: true })).toEqual({
      ...photo("image/jpeg"),
      width: 4,
      height: 2,
    });
    const bigger = { ...file, compressed: { ...compressed, bytes: new Uint8Array(20) } };
    expect(versionToSend(bigger).mimeType).toBe("image/jpeg");
    expect(versionToSend(photo("image/gif"))).toEqual(photo("image/gif"));
  });
});

describe("the default for new photos", () => {
  const storage = (stored: string | undefined) => ({
    get: () => Promise.resolve(stored),
    set: () => Promise.resolve(),
    remove: () => Promise.resolve(),
  });

  it("reads the stored choice, or compresses on mobile data", async () => {
    expect(await readMediaQuality(storage(undefined))).toBe("auto");
    expect(await readMediaQuality(storage("nonsense"))).toBe("auto");
    expect(await readMediaQuality(storage("original"))).toBe("original");
    expect(await compressByDefault("compress")).toBe(true);
    expect(await compressByDefault("original")).toBe(false);
    jest.mocked(getNetworkStateAsync).mockResolvedValueOnce({ type: NetworkStateType.CELLULAR });
    expect(await compressByDefault("auto")).toBe(true);
  });

  it("knows mobile data on phones, and never on the web or the desktop", async () => {
    jest.mocked(getNetworkStateAsync).mockResolvedValueOnce({ type: NetworkStateType.WIFI });
    expect(await onMobileData()).toBe(false);
    jest.mocked(getNetworkStateAsync).mockRejectedValueOnce(new Error("no permission"));
    expect(await onMobileData()).toBe(false);
    expect(await onMobileDataOnWeb()).toBe(false);
  });
});
