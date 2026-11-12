/**
 * Link previews in the browser and the desktop app. The desktop shell
 * fetches for the page, so the instance never sees the link; a plain
 * browser can only ask the instance's preview proxy, and only when the
 * admin turned it on. Otherwise messages go out without previews.
 */
import { getLinkPreview, getLinkPreviewImage, type Client } from "@nexus-chat/api-client";

import type { PreviewFetcher, PreviewSource } from "../messaging/previewFetch";

interface DesktopPreviews {
  fetchPreview?: (
    url: string,
    kind: "page" | "image",
  ) => Promise<{ contentType: string; bytes: Uint8Array } | undefined>;
}

/** A page through the proxy, rebuilt as the head the parser reads. */
function proxied(client: Client): PreviewFetcher {
  const escape = (text: string): string => text.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
  return async (url, kind) => {
    if (kind === "image") {
      const image = await getLinkPreviewImage({ client, query: { url }, parseAs: "blob" });
      const blob = image.data;
      return blob === undefined
        ? undefined
        : { contentType: blob.type, bytes: new Uint8Array(await blob.arrayBuffer()) };
    }
    const { data } = await getLinkPreview({ client, query: { url } });
    if (data === undefined) {
      return undefined;
    }
    const meta = (name: string, value: string | undefined): string =>
      value === undefined || value === "" ? "" : `<meta property="${name}" content="${escape(value)}">`;
    const head =
      meta("og:title", data.title) +
      meta("og:description", data.description) +
      meta("og:image", data.imageUrl);
    return { contentType: "text/html", bytes: new TextEncoder().encode(`<head>${head}</head>`) };
  };
}

/** How this page makes link previews, or undefined when it cannot. */
export function previewSource(client: Client | undefined, proxyEnabled: boolean): PreviewSource | undefined {
  const desktop = (globalThis as { nexusDesktop?: DesktopPreviews }).nexusDesktop?.fetchPreview;
  if (desktop !== undefined) {
    return {
      fetch: async (url, kind) => {
        const resource = await desktop(url, kind);
        return resource === undefined
          ? undefined
          : { contentType: resource.contentType, bytes: new Uint8Array(resource.bytes) };
      },
      viaInstance: false,
    };
  }
  return client !== undefined && proxyEnabled ? { fetch: proxied(client), viaInstance: true } : undefined;
}
