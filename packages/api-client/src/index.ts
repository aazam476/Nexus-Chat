/**
 * Typed client for the Nexus Chat server API.
 *
 * Every operation and type is generated from `apps/server/api/openapi.yaml`
 * into `src/generated` by `pnpm generate`. This file only adds
 * {@link createApiClient}, which builds a client bound to one instance.
 *
 * @example
 * ```ts
 * const client = createApiClient({ baseUrl: "https://chat.example.com" });
 * const { data } = await getReadiness({ client });
 * ```
 *
 * @packageDocumentation
 */
import { createClient, createConfig, type Client } from "./generated/client/index.js";

export * from "./generated/sdk.gen.js";
export type * from "./generated/types.gen.js";
export type { Client } from "./generated/client/index.js";

/** Settings for {@link createApiClient}. */
export interface ApiClientOptions {
  /** The instance's base URL, for example `https://chat.example.com`. */
  baseUrl: string;
  /** Replaces the global `fetch`, for tests or custom networking. */
  fetch?: typeof fetch;
}

/**
 * Creates a client for one instance. Each call returns an independent
 * client, so an app connected to several instances never mixes them up.
 *
 * @throws {@link TypeError} if `baseUrl` is not an absolute http(s) URL.
 */
export function createApiClient(options: ApiClientOptions): Client {
  const config = createConfig({ baseUrl: normalizeBaseUrl(options.baseUrl) });
  if (options.fetch !== undefined) {
    config.fetch = options.fetch;
  }
  return createClient(config);
}

/**
 * Checks that `baseUrl` is an absolute http or https URL and removes any
 * trailing slash, so paths join as `https://host/readyz` rather than
 * `https://host//readyz`.
 */
function normalizeBaseUrl(baseUrl: string): string {
  let parsed: URL;
  try {
    parsed = new URL(baseUrl);
  } catch {
    throw new TypeError(`baseUrl "${baseUrl}" is not an absolute URL`);
  }
  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    throw new TypeError(`baseUrl "${baseUrl}" must use http or https`);
  }
  return parsed.href.replace(/\/+$/, "");
}
