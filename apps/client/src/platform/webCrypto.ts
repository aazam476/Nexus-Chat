/**
 * WebCrypto where the JavaScript engine has none. Hermes, the engine of the
 * Android and iOS apps, has no `crypto` object, and the MLS, HPKE and
 * attachment code needs `crypto.subtle` (Ed25519, X25519, AES-GCM, HKDF,
 * SHA-2) and `crypto.getRandomValues` as soon as it loads.
 * react-native-quick-crypto implements them natively with OpenSSL and the
 * operating system's random generator. Browsers and Electron keep their own.
 */
import QuickCrypto from "react-native-quick-crypto";

interface CryptoLike {
  subtle?: unknown;
  getRandomValues?: unknown;
  randomUUID?: unknown;
}

/** Puts native WebCrypto on `target` unless it already has a complete one. */
export function installWebCrypto(target: { crypto?: unknown } = globalThis): void {
  const existing = target.crypto as CryptoLike | undefined;
  const complete =
    existing?.subtle !== undefined &&
    typeof existing.getRandomValues === "function" &&
    typeof existing.randomUUID === "function";
  if (!complete) {
    target.crypto = QuickCrypto;
  }
}

installWebCrypto();
