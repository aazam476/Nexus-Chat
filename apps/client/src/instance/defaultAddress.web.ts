/**
 * The web build is served by an instance, which is the one to use (spec
 * §2). Electron loads the same build from its own files, so it starts with
 * none and asks.
 */
export function defaultAddress(): string | undefined {
  if (typeof window === "undefined" || !window.location.protocol.startsWith("http")) {
    return undefined;
  }
  return window.location.origin;
}
