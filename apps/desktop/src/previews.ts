/**
 * Fetching a link preview's page or image for the app (spec §4.5): the
 * desktop app fetches previews itself, like a browser would, so the
 * instance never sees the links. Only http and https, with size limits:
 * enough of a page to read its head, and a modest image.
 */

/** What the app asks for. */
export type PreviewKind = "page" | "image";

/** A fetched page (as text) or image (as bytes). */
export interface PreviewResource {
  contentType: string;
  bytes: Uint8Array;
}

const limits: Record<PreviewKind, number> = { page: 1 << 20, image: 5 << 20 };
const imageTypes = new Set(["image/png", "image/jpeg", "image/gif", "image/webp"]);

function accepted(kind: PreviewKind, contentType: string): boolean {
  return kind === "page"
    ? contentType === "text/html" || contentType === "application/xhtml+xml"
    : imageTypes.has(contentType);
}

/**
 * Fetches `url` with `fetcher` (Electron's net.fetch in the app), or
 * returns undefined for anything that is not a public web page or image.
 */
export async function fetchPreviewResource(
  fetcher: (url: string, init: RequestInit) => Promise<Response>,
  url: unknown,
  kind: unknown,
): Promise<PreviewResource | undefined> {
  if (typeof url !== "string" || (kind !== "page" && kind !== "image") || !/^https?:\/\//i.test(url)) {
    return undefined;
  }
  try {
    const response = await fetcher(url, { redirect: "follow", signal: AbortSignal.timeout(8000) });
    const contentType = (response.headers.get("content-type") ?? "")
      .replace(/;.*$/s, "")
      .trim()
      .toLowerCase();
    if (!response.ok || !accepted(kind, contentType)) {
      return undefined;
    }
    const bytes = new Uint8Array(await response.arrayBuffer());
    if (kind === "image" && bytes.length > limits.image) {
      return undefined;
    }
    return { contentType, bytes: bytes.slice(0, limits[kind]) };
  } catch {
    return undefined;
  }
}
