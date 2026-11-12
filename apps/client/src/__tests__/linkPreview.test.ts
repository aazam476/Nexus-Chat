import { firstLink, linksOf, parsePreviewHtml } from "../messaging/linkPreview";
import { buildPreview, fetchDirect, previewTimeoutMs, type PreviewSource } from "../messaging/previewFetch";

const page = (head: string): Uint8Array<ArrayBuffer> =>
  new TextEncoder().encode(
    `<html><head>${head}</head><body><meta property="og:title" content="body"></body></html>`,
  );

describe("finding the link to preview", () => {
  it("takes the first web link outside code", () => {
    expect(
      firstLink("`https://code.example` then [docs](https://docs.example/a) and https://b.example"),
    ).toBe("https://docs.example/a");
    expect(linksOf("mail mailto:a@b.example and https://b.example")).toEqual(["https://b.example"]);
    expect(firstLink("```\nhttps://code.example\n```")).toBeUndefined();
    expect(firstLink("no links")).toBeUndefined();
  });
});

describe("reading a page's preview", () => {
  it("prefers Open Graph, then Twitter, then the title, and resolves the image", () => {
    const html = `<title>Plain &amp; simple</title>
      <meta name="description" content="Fallback">
      <meta property="og:title" content="Rich &quot;title&quot;">
      <meta property="og:title" content="Second">
      <meta name="twitter:description" content='From Twitter'>
      <meta property="og:image" content="/cover.png">`;
    expect(parsePreviewHtml(new TextDecoder().decode(page(html)), "https://site.example/post")).toEqual({
      title: 'Rich "title"',
      description: "From Twitter",
      imageUrl: "https://site.example/cover.png",
    });
    expect(parsePreviewHtml("<title> Only </title>", "https://x.example")).toEqual({ title: "Only" });
  });

  it("drops images that are not on the web and shortens long text", () => {
    const html = `<meta property="og:image" content="javascript:alert(1)"><meta property="og:description" content="${"a".repeat(1200)}">`;
    const found = parsePreviewHtml(html, "https://x.example");
    expect(found.imageUrl).toBeUndefined();
    expect(found.description).toHaveLength(1000);
    expect(found.description?.endsWith("…")).toBe(true);
    expect(parsePreviewHtml('<meta property="og:image" content="http://[bad">', "https://x.example")).toEqual(
      {},
    );
  });
});

describe("fetching a preview directly", () => {
  const respond = (body: Uint8Array<ArrayBuffer>, type: string, status = 200): void => {
    jest
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(body, { status, headers: { "content-type": type } }));
  };
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("accepts HTML pages and small images only", async () => {
    respond(page("<title>Hi</title>"), "text/html; charset=utf-8");
    expect((await fetchDirect("https://x.example", "page"))?.contentType).toBe("text/html");
    respond(new Uint8Array([1]), "image/svg+xml");
    expect(await fetchDirect("https://x.example/a.svg", "image")).toBeUndefined();
    respond(new Uint8Array(5 * 1024 * 1024 + 1), "image/png");
    expect(await fetchDirect("https://x.example/big.png", "image")).toBeUndefined();
    respond(new Uint8Array([1, 2]), "image/png");
    expect((await fetchDirect("https://x.example/a.png", "image"))?.bytes).toEqual(new Uint8Array([1, 2]));
    respond(page(""), "text/html", 404);
    expect(await fetchDirect("https://x.example/missing", "page")).toBeUndefined();
    respond(new Uint8Array(2 << 20), "text/html");
    expect((await fetchDirect("https://x.example/long", "page"))?.bytes.length).toBe(1 << 20);
    expect(await fetchDirect("file:///etc/passwd", "page")).toBeUndefined();
  });
});

describe("building a message's preview", () => {
  const reference = {
    id: "img",
    name: "preview.png",
    mimeType: "image/png",
    size: 2,
    scheme: "aes-256-gcm",
    key: "k",
    ciphertextSha256: "h",
  };
  const source = (pages: Record<string, Uint8Array<ArrayBuffer>>): PreviewSource => ({
    viaInstance: false,
    fetch: (url, kind) =>
      Promise.resolve(
        pages[url] === undefined
          ? undefined
          : { contentType: kind === "page" ? "text/html" : "image/png", bytes: pages[url] },
      ),
  });

  it("uploads the image and carries the page's text", async () => {
    const upload = jest.fn().mockResolvedValue(reference);
    const preview = await buildPreview(
      "https://x.example",
      source({
        "https://x.example": page(
          '<meta property="og:title" content="Title"><meta property="og:image" content="https://x.example/i.png">',
        ),
        "https://x.example/i.png": new Uint8Array([1, 2]),
      }),
      upload,
    );
    expect(preview).toEqual({ url: "https://x.example", title: "Title", image: reference });
    expect(upload).toHaveBeenCalledWith({
      name: "preview.png",
      mimeType: "image/png",
      bytes: new Uint8Array([1, 2]),
    });
  });

  it("gives up without a page, without text, on failure or when slow", async () => {
    const upload = jest.fn();
    expect(await buildPreview("https://x.example", source({}), upload)).toBeUndefined();
    expect(
      await buildPreview("https://x.example", source({ "https://x.example": page("") }), upload),
    ).toBeUndefined();
    const broken: PreviewSource = { viaInstance: false, fetch: () => Promise.reject(new Error("offline")) };
    expect(await buildPreview("https://x.example", broken, upload)).toBeUndefined();
    jest.useFakeTimers();
    const slow: PreviewSource = { viaInstance: false, fetch: () => new Promise(() => undefined) };
    const pending = buildPreview("https://x.example", slow, upload);
    jest.advanceTimersByTime(previewTimeoutMs);
    expect(await pending).toBeUndefined();
    jest.useRealTimers();
    expect(upload).not.toHaveBeenCalled();
  });
});
