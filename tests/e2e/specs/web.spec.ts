/**
 * The release gate's smoke test (spec §7.4) in a real browser: setup
 * wizard → admin → second user, with several people and devices at once.
 * Two people then message each other, end-to-end encrypted; voice join
 * comes with milestone M9.
 */
import { expect, test } from "@playwright/test";

import { expectAccessible } from "../src/accessibility.js";
import {
  becomeFriends,
  restoreWithPhrase,
  sendMessage,
  shown,
  signUpForMessaging,
} from "../src/messaging.js";
import { expectHome, field, fillAccount, logIn } from "../src/screens.js";
import { accounts, settings } from "../src/settings.js";

test.describe.configure({ mode: "serial" });

test("the setup wizard creates the admin", async ({ page }) => {
  expect(settings.setupToken, "NEXUS_E2E_SETUP_TOKEN must be set").not.toBe("");
  await page.goto("/");
  await expect(page.getByTestId("setup-submit")).toBeVisible();
  await expect(page.getByTestId("setup-http-warning")).toHaveCount(0); // localhost or HTTPS
  await expectAccessible(page);

  await field(page, "Setup token").fill(settings.setupToken);
  await fillAccount(page, accounts.admin);
  await field(page, "Instance name").fill("Release Gate");
  await page.getByTestId("mode-open").click();
  await page.getByTestId("setup-submit").click();

  await expect(page.getByTestId("setup-go-login")).toBeVisible();
  await expectAccessible(page);
  await page.getByTestId("setup-go-login").click();
  await logIn(page, accounts.admin);
  await expectAccessible(page);
});

test("the admin opens the Admin panel", async ({ page }) => {
  await page.goto("/");
  await logIn(page, accounts.admin);
  await page.getByTestId("go-admin").click();

  await expect(page.getByTestId("tls-state")).toHaveText(/No certificate is installed/);
  await expect(page.getByTestId("smtp-host")).toBeVisible();
  await expectAccessible(page);
  await page.getByTestId("smtp-test").click();
  await expect(page.getByText("Save the email settings before sending a test email.")).toBeVisible();
});

test("a second user signs up", async ({ page }) => {
  await page.goto("/");
  await page.getByTestId("go-register").click();
  await expect(page.getByTestId("register-submit")).toBeVisible();
  await expectAccessible(page);

  await fillAccount(page, accounts.member);
  await page.getByTestId("register-submit").click();

  await expectHome(page, accounts.member);
});

test("two people message each other, end to end encrypted", async ({ browser }) => {
  const contexts = await Promise.all([browser.newContext(), browser.newContext()]);
  const [lin, omar] = await Promise.all(contexts.map((context) => context.newPage()));
  if (lin === undefined || omar === undefined) {
    throw new Error("could not open the devices");
  }
  const linPhrase = await signUpForMessaging(lin, accounts.lin);
  await signUpForMessaging(omar, accounts.omar);
  await becomeFriends({ page: lin, account: accounts.lin }, { page: omar, account: accounts.omar });

  await sendMessage(lin, "Hello from the release gate");
  await expect(omar.getByText("Hello from the release gate").filter({ visible: true })).toBeVisible({
    timeout: 30_000,
  });

  // Typing shows on the other side while the reply is written.
  await shown(omar, "composer").pressSequentially("Hi");
  await expect(shown(lin, "typing")).toBeVisible({ timeout: 10_000 });
  await sendMessage(omar, "Hi Lin, it works");
  await expect(lin.getByText("Hi Lin, it works").filter({ visible: true })).toBeVisible({ timeout: 30_000 });
  await expectAccessible(lin);

  // A second device restores with the phrase and reads the conversation.
  const tabletContext = await browser.newContext();
  const tablet = await tabletContext.newPage();
  await tablet.goto("/");
  await logIn(tablet, accounts.lin);
  await restoreWithPhrase(tablet, linPhrase);
  await shown(tablet, "open-messages").click();
  await tablet
    .getByTestId(/^conversation-/)
    .filter({ visible: true })
    .first()
    .click();
  await expect(tablet.getByText("Hi Lin, it works").filter({ visible: true })).toBeVisible({
    timeout: 30_000,
  });
  await Promise.all([...contexts, tabletContext].map((context) => context.close()));
});

test("status changes reach the user's other devices at once", async ({ browser }) => {
  // Each device is its own browser context, so it holds its own session.
  const contexts = await Promise.all([browser.newContext(), browser.newContext()]);
  const [laptop, phone] = await Promise.all(contexts.map((context) => context.newPage()));
  if (laptop === undefined || phone === undefined) {
    throw new Error("could not open the devices");
  }
  for (const device of [laptop, phone]) {
    await device.goto("/");
    await logIn(device, accounts.admin);
  }

  await laptop.getByTestId("status-dnd").click();
  await expect(phone.getByTestId("presence")).toHaveText("Others see you as: Do not disturb");

  await laptop.getByTestId("status-online").click();
  await expect(phone.getByTestId("presence")).toHaveText("Others see you as: Online");
  await Promise.all(contexts.map((context) => context.close()));
});

test("the session survives a reload and ends at logout", async ({ page }) => {
  await page.goto("/");
  await logIn(page, accounts.member);
  await page.reload();
  await expectHome(page, accounts.member);

  await page.getByTestId("logout").click();
  await expect(page.getByTestId("login-submit")).toBeVisible();
  await page.reload();
  await expect(page.getByTestId("login-submit")).toBeVisible();
});

test("a wrong password is explained", async ({ page }) => {
  await page.goto("/");
  await field(page, "Username or email").fill(accounts.member.username);
  await field(page, "Password").fill("not the password");
  await page.getByTestId("login-submit").click();

  await expect(page.getByText("The username or password is wrong.")).toBeVisible();
  await expectAccessible(page);
});
