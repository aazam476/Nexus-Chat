import { createApiClient } from "@nexus-chat/api-client";

import { parsePreviewHtml } from "../messaging/linkPreview";
import { previewSource as nativeSource } from "../platform/previewSource";
import { previewSource } from "../platform/previewSource.web";

const json = (body: unknown): Response =>
  new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } });

describe("where link previews come from", () => {
  afterEach(() => {
    delete (globalThis as { nexusDesktop?: unknown }).nexusDesktop;
  });

  it("lets phones fetch links themselves", () => {
    expect(nativeSource(undefined, false)?.viaInstance).toBe(false);
  });

  it("uses the desktop shell before the instance", async () => {
    const fetchPreview = jest
      .fn()
      .mockResolvedValueOnce({ contentType: "text/html", bytes: new Uint8Array([60]) })
      .mockResolvedValueOnce(undefined);
    Object.assign(globalThis, { nexusDesktop: { fetchPreview } });
    const source = previewSource(undefined, true);
    expect(source?.viaInstance).toBe(false);
    expect(await source?.fetch("https://x.example", "page")).toEqual({
      contentType: "text/html",
      bytes: new Uint8Array([60]),
    });
    expect(await source?.fetch("https://x.example/i.png", "image")).toBeUndefined();
    expect(fetchPreview).toHaveBeenCalledWith("https://x.example", "page");
  });

  it("asks the instance only when its proxy is on", async () => {
    const requests: string[] = [];
    const fetch = jest.fn((request: Request) => {
      requests.push(request.url);
      if (request.url.includes("/image")) {
        return Promise.resolve(new Response(new Blob([new Uint8Array([1, 2])], { type: "image/png" })));
      }
      return Promise.resolve(
        request.url.includes("missing")
          ? new Response(JSON.stringify({ code: "not_found", message: "no" }), { status: 404 })
          : json({ title: 'A "post"', description: "", imageUrl: "https://x.example/i.png" }),
      );
    });
    const client = createApiClient({
      baseUrl: "https://chat.example",
      fetch: fetch as unknown as typeof globalThis.fetch,
    });
    expect(previewSource(client, false)).toBeUndefined();
    expect(previewSource(undefined, true)).toBeUndefined();
    const source = previewSource(client, true);
    expect(source?.viaInstance).toBe(true);

    const page = await source?.fetch("https://x.example/a", "page");
    expect(parsePreviewHtml(new TextDecoder().decode(page?.bytes), "https://x.example/a")).toEqual({
      title: 'A "post"',
      imageUrl: "https://x.example/i.png",
    });
    expect(await source?.fetch("https://x.example/missing", "page")).toBeUndefined();
    expect(await source?.fetch("https://x.example/i.png", "image")).toEqual({
      contentType: "image/png",
      bytes: new Uint8Array([1, 2]),
    });
    expect(requests[0]).toBe("https://chat.example/api/v1/link-previews?url=https%3A%2F%2Fx.example%2Fa");
  });
});
