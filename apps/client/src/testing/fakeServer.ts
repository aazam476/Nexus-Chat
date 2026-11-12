/**
 * A fake instance for tests: answers API requests from a table of routes,
 * and records every request it got.
 */
import { createApiClient, type Client } from "@nexus-chat/api-client";

/** One canned answer. */
export interface FakeAnswer {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
}

/** A fake server and the client that talks to it. */
export interface FakeServer {
  client: Client;
  requests: { method: string; path: string; body: unknown; authorization: string | null }[];
  /** Sets the answer for "METHOD /path". */
  answer: (route: string, answer: FakeAnswer) => void;
  fetch: typeof fetch;
}

/** A request body as JSON, or as the raw text for uploads that are not JSON. */
function parsedBody(text: string): unknown {
  if (text === "") {
    return undefined;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

/** Creates a fake server; unknown routes answer 404. */
export function fakeServer(token?: () => string | undefined): FakeServer {
  const routes = new Map<string, FakeAnswer>();
  const requests: FakeServer["requests"] = [];
  const fakeFetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const request = new Request(input, init);
    const path = new URL(request.url).pathname;
    const text = await request.text();
    requests.push({
      method: request.method,
      path,
      body: parsedBody(text),
      authorization: request.headers.get("Authorization"),
    });
    const answer = routes.get(`${request.method} ${path}`) ?? {
      status: 404,
      body: { code: "not_found", message: "no route" },
    };
    const headers = { "Content-Type": "application/json", ...answer.headers };
    return new Response(answer.body === undefined ? null : JSON.stringify(answer.body), {
      status: answer.status,
      headers,
    });
  };
  return {
    client: createApiClient({
      baseUrl: "https://chat.example.com",
      fetch: fakeFetch,
      ...(token === undefined ? {} : { token }),
    }),
    requests,
    answer: (route, answer) => {
      routes.set(route, answer);
    },
    fetch: fakeFetch,
  };
}
