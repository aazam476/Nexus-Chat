import { createApiClient, getLiveness, getReadiness, type HealthStatus } from "./index.js";

/** A fetch replacement that records requests and answers with `status` and `body`. */
function fakeFetch(status: number, body: HealthStatus): { fetch: typeof fetch; requests: Request[] } {
  const requests: Request[] = [];
  const fetchImplementation = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    requests.push(new Request(input, init));
    return Promise.resolve(
      new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }),
    );
  };
  return { fetch: fetchImplementation, requests };
}

describe("createApiClient", () => {
  it("sends requests to the instance's base URL", async () => {
    const fake = fakeFetch(200, { status: "ok", checks: [] });
    const client = createApiClient({ baseUrl: "https://chat.example.com/", fetch: fake.fetch });

    const result = await getLiveness({ client });

    expect(fake.requests.map((request) => request.url)).toEqual(["https://chat.example.com/healthz"]);
    expect(result.data).toEqual({ status: "ok", checks: [] });
  });

  it("keeps a base path and drops repeated trailing slashes", async () => {
    const fake = fakeFetch(200, { status: "ok", checks: [] });
    const client = createApiClient({ baseUrl: "http://localhost:8080/nexus//", fetch: fake.fetch });

    await getLiveness({ client });

    expect(fake.requests[0]?.url).toBe("http://localhost:8080/nexus/healthz");
  });

  it("reports a 503 readiness answer as an error with the documented body", async () => {
    const body: HealthStatus = {
      status: "unavailable",
      checks: [{ name: "database", status: "unavailable" }],
    };
    const fake = fakeFetch(503, body);
    const client = createApiClient({ baseUrl: "https://chat.example.com", fetch: fake.fetch });

    const result = await getReadiness({ client });

    expect(result.response?.status).toBe(503);
    expect(result.error).toEqual(body);
    expect(result.data).toBeUndefined();
  });

  it("returns independent clients", async () => {
    const first = fakeFetch(200, { status: "ok", checks: [] });
    const second = fakeFetch(200, { status: "ok", checks: [] });
    const firstClient = createApiClient({ baseUrl: "https://one.example.com", fetch: first.fetch });
    const secondClient = createApiClient({ baseUrl: "https://two.example.com", fetch: second.fetch });

    await getLiveness({ client: firstClient });
    await getLiveness({ client: secondClient });

    expect(first.requests[0]?.url).toBe("https://one.example.com/healthz");
    expect(second.requests[0]?.url).toBe("https://two.example.com/healthz");
  });

  it("uses the global fetch when none is given", () => {
    const client = createApiClient({ baseUrl: "https://chat.example.com" });

    expect(client.getConfig().fetch).toBeUndefined();
    expect(client.getConfig().baseUrl).toBe("https://chat.example.com");
  });

  it.each([
    ["chat.example.com", "is not an absolute URL"],
    ["", "is not an absolute URL"],
    ["ftp://chat.example.com", "must use http or https"],
    ["javascript:alert(1)", "must use http or https"],
  ])("rejects base URL %p", (baseUrl, message) => {
    expect(() => createApiClient({ baseUrl })).toThrow(new TypeError(`baseUrl "${baseUrl}" ${message}`));
  });
});
