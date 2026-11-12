/**
 * Where a link preview's page and image come from, and building the
 * preview a message carries. Phones fetch the link themselves; browsers
 * cannot (other sites refuse them), so the desktop app fetches for its
 * page and a plain browser asks the instance, only when the admin allows
 * it (docs/e2ee-design.md §8).
 */
import type { PickedFile } from "./attachmentTransfer";
import { parsePreviewHtml } from "./linkPreview";
import type { AttachmentReference, LinkPreview } from "./payload";

/** One fetched resource. */
export interface PreviewResource {
  contentType: string;
  bytes: Uint8Array<ArrayBuffer>;
}

/** Fetches the page or image of a link, or undefined when it cannot. */
export type PreviewFetcher = (url: string, kind: "page" | "image") => Promise<PreviewResource | undefined>;

/** How this device makes previews; `viaInstance` means the instance sees the link. */
export interface PreviewSource {
  fetch: PreviewFetcher;
  viaInstance: boolean;
}

const limits = { page: 1 << 20, image: 5 << 20 };
const imageTypes = ["image/png", "image/jpeg", "image/gif", "image/webp"];
/** How long making a preview may hold up sending. */
export const previewTimeoutMs = 4000;

/** Fetches a resource straight from its site, within the preview limits. */
export async function fetchDirect(url: string, kind: "page" | "image"): Promise<PreviewResource | undefined> {
  if (!/^https?:\/\//i.test(url)) {
    return undefined;
  }
  const response = await fetch(url, { signal: AbortSignal.timeout(previewTimeoutMs) });
  const contentType = (response.headers.get("content-type") ?? "").split(";")[0]?.trim().toLowerCase() ?? "";
  const fits = kind === "page" ? contentType === "text/html" : imageTypes.includes(contentType);
  if (!response.ok || !fits) {
    return undefined;
  }
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (kind === "image" && bytes.length > limits.image) {
    return undefined;
  }
  return { contentType, bytes: kind === "page" ? bytes.slice(0, limits.page) : bytes };
}

async function previewOf(
  url: string,
  source: PreviewSource,
  upload: (file: PickedFile) => Promise<AttachmentReference>,
): Promise<LinkPreview | undefined> {
  const page = await source.fetch(url, "page");
  if (page === undefined) {
    return undefined;
  }
  const found = parsePreviewHtml(new TextDecoder().decode(page.bytes), url);
  if (found.title === undefined && found.description === undefined) {
    return undefined;
  }
  const image = found.imageUrl === undefined ? undefined : await source.fetch(found.imageUrl, "image");
  const extension = image?.contentType.split("/")[1] ?? "png";
  const reference =
    image === undefined
      ? undefined
      : await upload({ name: `preview.${extension}`, mimeType: image.contentType, bytes: image.bytes });
  return {
    url,
    ...(found.title === undefined ? {} : { title: found.title }),
    ...(found.description === undefined ? {} : { description: found.description }),
    ...(reference === undefined ? {} : { image: reference }),
  };
}

/**
 * The preview for `url`, its image uploaded encrypted with `upload`, or
 * undefined if it cannot be made in time. Never throws: a message goes
 * out without a preview rather than not at all.
 */
export async function buildPreview(
  url: string,
  source: PreviewSource,
  upload: (file: PickedFile) => Promise<AttachmentReference>,
): Promise<LinkPreview | undefined> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const late = new Promise<undefined>((resolve) => {
    timer = setTimeout(resolve, previewTimeoutMs);
  });
  try {
    return await Promise.race([previewOf(url, source, upload), late]);
  } catch {
    return undefined;
  } finally {
    clearTimeout(timer);
  }
}
