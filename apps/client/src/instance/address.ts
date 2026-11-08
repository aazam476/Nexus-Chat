/**
 * Turns what a user types ("chat.example.com") into an instance's base
 * address ("https://chat.example.com").
 */

const loopbackHosts = new Set(["localhost", "127.0.0.1", "[::1]"]);

/**
 * The base address for `input`, or undefined when it is not one. Without a
 * scheme, HTTPS is assumed, except for this computer, where instances
 * usually run over plain HTTP while testing.
 */
export function normalizeAddress(input: string): string | undefined {
  const trimmed = input.trim();
  if (trimmed === "" || /\s/.test(trimmed)) {
    return undefined;
  }
  const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed)
    ? trimmed
    : `${schemeFor(trimmed)}://${trimmed}`;
  let parsed: URL;
  try {
    parsed = new URL(withScheme);
  } catch {
    return undefined;
  }
  if ((parsed.protocol !== "https:" && parsed.protocol !== "http:") || parsed.hostname === "") {
    return undefined;
  }
  return parsed.origin;
}

function schemeFor(address: string): "http" | "https" {
  const host = address.split("/")[0]?.split(":")[0] ?? "";
  return loopbackHosts.has(host) || address.startsWith("[::1]") ? "http" : "https";
}

/** The host part of an address, for showing to the user. */
export function hostOf(address: string): string {
  return new URL(address).host;
}
