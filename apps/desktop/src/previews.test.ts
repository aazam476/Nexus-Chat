import { fetchPreviewResource } from "./previews";

const respond =
  (body: BodyInit, contentType: string, status = 200) =>
  () =>
    Promise.resolve(new Response(body, { status, headers: { "Content-Type": contentType } }));

describe("fetching for link previews", () => {
  it("returns a page or an image", async () => {
    const page = await fetchPreviewResource(
      respond("<title>x</title>", "text/html; charset=utf-8"),
      "https://example.com",
      "page",
    );
    expect(page?.contentType).toBe("text/html");
    expect(new TextDecoder().decode(page?.bytes)).toBe("<title>x</title>");
    const image = await fetchPreviewResource(
      respond(new Uint8Array([1, 2]), "image/png"),
      "http://example.com/a.png",
      "image",
    );
    expect(image).toEqual({ contentType: "image/png", bytes: new Uint8Array([1, 2]) });
  });

  it("cuts a long page to its first MiB and refuses large images", async () => {
    const long = await fetchPreviewResource(
      respond("a".repeat((1 << 20) + 5), "text/html"),
      "https://example.com",
      "page",
    );
    expect(long?.bytes.length).toBe(1 << 20);
    const large = await fetchPreviewResource(
      respond(new Uint8Array((5 << 20) + 1), "image/jpeg"),
      "https://example.com",
      "image",
    );
    expect(large).toBeUndefined();
  });

  it("refuses anything else", async () => {
    const fetcher = jest.fn(respond("<title>x</title>", "text/html"));
    for (const [url, kind] of [
      ["file:///etc/passwd", "page"],
      [42, "page"],
      ["https://example.com", "script"],
    ] as const) {
      expect(await fetchPreviewResource(fetcher, url, kind)).toBeUndefined();
    }
    expect(fetcher).not.toHaveBeenCalled();
    expect(
      await fetchPreviewResource(respond("x", "text/plain"), "https://example.com", "page"),
    ).toBeUndefined();
    expect(
      await fetchPreviewResource(respond("x", "image/svg+xml"), "https://example.com", "image"),
    ).toBeUndefined();
    expect(
      await fetchPreviewResource(respond("gone", "text/html", 404), "https://example.com", "page"),
    ).toBeUndefined();
    expect(
      await fetchPreviewResource(() => Promise.reject(new Error("offline")), "https://example.com", "page"),
    ).toBeUndefined();
  });
});
