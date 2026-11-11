/**
 * A secure random source where the JavaScript engine has none. Hermes, the
 * engine of the Android and iOS apps, has no `crypto.getRandomValues`, and
 * the crypto libraries need it as soon as they load; expo-crypto gives the
 * operating system's generator. Browsers and Electron keep their own.
 */
import { getRandomValues } from "expo-crypto";

/** Adds `crypto.getRandomValues` to `target` if it is missing. */
export function installSecureRandom(target: { crypto?: unknown } = globalThis): void {
  const existing = target.crypto as { getRandomValues?: unknown } | undefined;
  if (typeof existing?.getRandomValues === "function") {
    return;
  }
  target.crypto = Object.assign(existing ?? {}, { getRandomValues });
}

installSecureRandom();
