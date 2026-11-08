/**
 * The desktop app (spec §7.2) against the instance the web test set up:
 * pick the instance, log in, and confirm the page cannot reach Node.js or
 * leave the app. Needs `pnpm --filter @nexus-chat/desktop build` and the
 * client's web build first.
 */
import { _electron as electron, expect, test, type ElectronApplication, type Page } from "@playwright/test";
import { mkdtemp } from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { getuid } from "node:process";
import { fileURLToPath } from "node:url";

import { expectAccessible } from "../src/accessibility.js";
import { field, logIn } from "../src/screens.js";
import { accounts, settings } from "../src/settings.js";

const desktopDirectory = fileURLToPath(new URL("../../../apps/desktop/", import.meta.url));

let app: ElectronApplication;
let appWindow: Page;

test.describe.configure({ mode: "serial" });
test.skip(!settings.desktop, "set NEXUS_E2E_DESKTOP=1 to run the desktop test");

test.beforeAll(async () => {
  // The electron package's main export is the path of its binary.
  const executablePath = createRequire(desktopDirectory)("electron") as string;
  // Chromium's sandbox cannot start as root (containers); CI runs as a normal user.
  const sandbox = getuid?.() === 0 ? ["--no-sandbox"] : [];
  // A new profile each run, so the app starts as on first install.
  const profile = await mkdtemp(path.join(tmpdir(), "nexus-desktop-"));
  app = await electron.launch({
    executablePath,
    args: [...sandbox, `--user-data-dir=${profile}`, desktopDirectory],
  });
  appWindow = await app.firstWindow();
});

test.afterAll(async () => {
  await app.close();
});

test("picks the instance and logs in", async () => {
  await expect(appWindow.getByRole("heading", { name: "Connect to an instance" })).toBeVisible();
  await expectAccessible(appWindow);
  await field(appWindow, "Instance address").fill(settings.baseUrl);
  await appWindow.getByTestId("connect").click();

  await logIn(appWindow, accounts.member);
  expect(appWindow.url()).toBe("app://nexus-chat/home");
});

test("the page has no Node.js and cannot navigate away", async () => {
  expect(await appWindow.evaluate(() => typeof (globalThis as { require?: unknown }).require)).toBe(
    "undefined",
  );
  expect(
    await appWindow.evaluate(
      () =>
        typeof (globalThis as { nexusDesktop?: { signInWithProvider?: unknown } }).nexusDesktop
          ?.signInWithProvider,
    ),
  ).toBe("function");

  await appWindow.evaluate(() => {
    window.location.href = "https://example.com/";
  });
  await appWindow.waitForTimeout(500); // a navigation that is allowed would have started by now
  expect(appWindow.url()).toBe("app://nexus-chat/home");
});
