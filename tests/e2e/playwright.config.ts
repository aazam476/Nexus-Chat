import { defineConfig } from "@playwright/test";

import { browserArguments, settings } from "./src/settings.js";

/** Where the static server for the visual checks listens. */
const visualPort = "4174";

/**
 * Two projects run against one fresh instance, in order: the browser smoke
 * test sets the instance up through the setup wizard, then the desktop
 * test logs in to it from the Electron app. Retries stay off so a flaky
 * step fails the run (spec §7.5).
 */
export default defineConfig({
  testDir: "specs",
  fullyParallel: false,
  workers: settings.workers,
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
    // Every screen with demo and worst-case data against a fake instance
    // (src/fakeInstance.ts), so it needs only the web build.
    {
      name: "visual",
      testMatch: "visual.spec.ts",
      fullyParallel: true,
      use: { baseURL: `http://127.0.0.1:${visualPort}` },
    },
  ],
  webServer: {
    command: `node src/staticServer.ts ../../apps/client/dist ${visualPort}`,
    url: `http://127.0.0.1:${visualPort}/`,
    reuseExistingServer: true,
  },
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.002 } },
});
