/** Link previews on phones: the device fetches the link itself. */
import type { Client } from "@nexus-chat/api-client";

import { fetchDirect, type PreviewSource } from "../messaging/previewFetch";

/** How this device makes link previews; phones never need the instance. */
export function previewSource(
  _client: Client | undefined,
  _proxyEnabled: boolean,
): PreviewSource | undefined {
  return { fetch: fetchDirect, viaInstance: false };
}
