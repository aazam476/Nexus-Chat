import { defineConfig } from "@playwright/test";

import { browserArguments, settings } from "./src/settings.js";

/**
 * Two projects run against one fresh instance, in order: the browser smoke
 * test sets the instance up through the setup wizard, then the desktop
 * test logs in to it from the Electron app. Retries stay off so a flaky
 * step fails the run (spec §7.5).
 */
export default defineConfig({
  testDir: "specs",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: true,
  timeout: 60_000,
  reporter: [["list"]],
  use: {
    baseURL: settings.baseUrl,
    trace: "off",
    launchOptions: { args: browserArguments(), executablePath: settings.chromiumPath },
  },
  projects: [
    { name: "web", testMatch: "web.spec.ts" },
    { name: "desktop", testMatch: "desktop.spec.ts", dependencies: ["web"] },
  ],
});
