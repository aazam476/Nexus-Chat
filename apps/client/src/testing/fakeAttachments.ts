/**
 * An attachment store for tests: a fetch function that keeps uploaded
 * ciphertext byte for byte and hands it back, so encryption round trips
 * can be checked through the real API client.
 */
import { createApiClient, type Client } from "@nexus-chat/api-client";

/** The fake store, its client, and what it holds. */
export interface FakeAttachments {
  client: Client;
  stored: Map<string, Uint8Array<ArrayBuffer>>;
  /** Answer the next upload with this status instead of storing it. */
  failNextUpload: () => void;
}

const upload = /^\/api\/v1\/conversations\/[^/]+\/attachments$/;
const download = /^\/api\/v1\/conversations\/[^/]+\/attachments\/([^/]+)$/;

/**
 * Creates an empty fake attachment store. Other requests go to `fallback`,
 * such as a fake server's fetch.
 */
export function fakeAttachments(fallback?: typeof fetch): FakeAttachments {
  const stored = new Map<string, Uint8Array<ArrayBuffer>>();
  let failing = false;
  const fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const request = new Request(input, init);
    const path = new URL(request.url).pathname;
    if (request.method === "POST" && upload.test(path)) {
      if (failing) {
        failing = false;
        return new Response(JSON.stringify({ code: "rate_limited", message: "slow down" }), { status: 429 });
      }
      const bytes = new Uint8Array(await request.arrayBuffer());
      const id = crypto.randomUUID();
      stored.set(id, bytes);
      return Response.json({ id, size: bytes.length }, { status: 201 });
    }
    const found = request.method === "GET" ? download.exec(path) : null;
    if (found === null && fallback !== undefined) {
      return fallback(request);
    }
    const bytes = found === null ? undefined : stored.get(found[1] ?? "");
    if (bytes === undefined) {
      return Response.json({ code: "attachment_not_found", message: "no" }, { status: 404 });
    }
    return new Response(bytes, { status: 200, headers: { "Content-Type": "application/octet-stream" } });
  };
  return {
    client: createApiClient({ baseUrl: "https://chat.example.com", fetch }),
    stored,
    failNextUpload: () => {
      failing = true;
    },
  };
}
