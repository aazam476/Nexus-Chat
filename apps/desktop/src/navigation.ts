/**
 * What the app window may load. The window shows only the bundled web
 * build; anything else opens in the user's browser, so a link in a message
 * can never take over the app window (Electron security checklist, items
 * 13 and 14).
 */
import { appOrigin } from "./files";

/** Whether the main window may navigate to `url`. */
export function isAppAddress(url: string): boolean {
  // Node.js gives custom schemes the origin "null", so compare the parts.
  try {
    const parsed = new URL(url);
    return `${parsed.protocol}//${parsed.host}` === appOrigin;
  } catch {
    return false;
  }
}

function hasProtocol(url: string, protocols: string[]): boolean {
  try {
    return protocols.includes(new URL(url).protocol);
  } catch {
    return false;
  }
}

/** Whether `url` may be handed to the system browser: web pages and mail only. */
export function isExternalLink(url: string): boolean {
  return hasProtocol(url, ["https:", "http:", "mailto:"]);
}

/** Whether `url` is a web page, the only thing a sign-in window may open. */
export function isWebPage(url: string): boolean {
  return hasProtocol(url, ["https:", "http:"]);
}

/**
 * Where the instance sends the provider sign-in window back to: the same
 * app address the mobile apps use, which the instance already accepts
 * (ADR 0016). The shell, not the page, decides it.
 */
export const providerReturnAddress = "nexuschat://oidc-return";

/**
 * The fragment of a redirect to the return address, or undefined while the
 * sign-in window is still at the provider. The address is matched exactly.
 */
export function providerReturnFragment(url: string): string | undefined {
  const [address, fragment] = url.split("#", 2);
  return address === providerReturnAddress ? (fragment ?? "") : undefined;
}

/**
 * The Content Security Policy for the bundled web build. Instances live at
 * any address, so the app may connect anywhere over HTTP(S) and WebSocket,
 * but scripts and styles come only from the bundle.
 */
export const contentSecurityPolicy =
  "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; " +
  "connect-src https: http: wss: ws:; font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'";
