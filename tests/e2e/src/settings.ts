/**
 * Everything the tests learn from the environment. The release gate
 * (scripts/release-gate.sh) sets these; a developer can point them at a
 * server started from apps/server.
 */
import { env } from "node:process";

/** The settings for one run. */
export const settings = {
  /** The instance, as a browser on this machine reaches it. */
  baseUrl: env.NEXUS_E2E_BASE_URL ?? "http://localhost:8080",
  /** The setup token from the server's logs; the instance must be fresh. */
  setupToken: env.NEXUS_E2E_SETUP_TOKEN ?? "",
  /** A Chromium to use instead of Playwright's own, if set. */
  chromiumPath: env.NEXUS_E2E_CHROMIUM,
  /** Chromium host resolver rules, so a test domain reaches this machine. */
  hostRules: env.NEXUS_E2E_HOST_RULES,
  /**
   * Base64 SHA-256 of the test certificate authority's public key, which
   * Chromium then trusts. The page stays a secure context, unlike with
   * certificate errors ignored.
   */
  trustedKey: env.NEXUS_E2E_TRUSTED_SPKI,
  /** Whether the desktop test runs; it needs a display and the Electron binary. */
  desktop: env.NEXUS_E2E_DESKTOP === "1",
};

/** Chromium command-line switches for the settings above. */
export function browserArguments(): string[] {
  const switches: string[] = [];
  if (settings.hostRules !== undefined) {
    switches.push(`--host-resolver-rules=${settings.hostRules}`);
  }
  if (settings.trustedKey !== undefined) {
    switches.push(`--ignore-certificate-errors-spki-list=${settings.trustedKey}`);
  }
  return switches;
}

/** The accounts the smoke test creates. */
export const accounts = {
  admin: {
    username: "admin",
    displayName: "Ada Admin",
    email: "admin@example.com",
    password: "release gate password",
  },
  member: {
    username: "grace",
    displayName: "Grace Member",
    email: "grace@example.com",
    password: "second user password",
  },
};
