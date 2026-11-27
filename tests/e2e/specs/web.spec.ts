/**
 * The release gate's smoke test (spec §7.4) in a real browser: setup
 * wizard → admin → second user, with several people and devices at once.
 * Two people then message each other, end-to-end encrypted, and a server's
 * channel carries messages to someone who joins it by invite; voice join
 * comes with milestone M9.
 */
import { expect, test, type Page } from "@playwright/test";

import { expectAccessible } from "../src/accessibility.js";
import {
  becomeFriends,
  react,
  replyTo,
  restoreWithPhrase,
  sendMessage,
  shown,
  signUpForMessaging,
} from "../src/messaging.js";
import { expectHome, field, fillAccount, logIn } from "../src/screens.js";
import {
  addChannel,
  addEmoji,
  replyInThread,
  channelOrder,
  dragChannelAbove,
  joinWith,
  makeInvite,
  makeServer,
  openChannel,
} from "../src/servers.js";
import { accounts, settings } from "../src/settings.js";

test.describe.configure({ mode: "serial" });

test("the setup wizard creates the admin", async ({ page }) => {
  expect(settings.setupToken, "NEXUS_E2E_SETUP_TOKEN must be set").not.toBe("");
  await page.goto("/");
  // The first load of a fresh instance can be slow on a busy CI machine.
  await expect(page.getByTestId("setup-submit")).toBeVisible({ timeout: 30_000 });
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

/** A reply quotes what it answers, and a reaction shows who reacted, on both sides. */
async function replyAndReact(lin: Page, omar: Page): Promise<void> {
  await replyTo(lin, "Hi Lin, it works", "Great, replying to you");
  await expect(
    omar.getByText(/^Reply to .*Omar Reader.*: Hi Lin, it works$/).filter({ visible: true }),
  ).toBeVisible({ timeout: 30_000 });
  await react(omar, "Great, replying to you", "👍");
  await expect(
    lin.getByRole("button", { name: /^👍 1, from .*Omar Reader/ }).filter({ visible: true }),
  ).toBeVisible({ timeout: 30_000 });
  await expectAccessible(lin);
}

test("two people message each other, end to end encrypted", async ({ browser }) => {
  test.setTimeout(180_000);
  const contexts = await Promise.all([browser.newContext(), browser.newContext()]);
  const [lin, omar] = await Promise.all(contexts.map((context) => context.newPage()));
  if (lin === undefined || omar === undefined) {
    throw new Error("could not open the devices");
  }
  const linPhrase = await signUpForMessaging(lin, accounts.lin);
  await signUpForMessaging(omar, accounts.omar);
  await becomeFriends({ page: lin, account: accounts.lin }, { page: omar, account: accounts.omar });

  await sendMessage(lin, "Hello from the release gate");
  await expect(
    omar.getByText("Hello from the release gate", { exact: true }).filter({ visible: true }),
  ).toBeVisible({
    timeout: 30_000,
  });

  // Typing shows on the other side while the reply is written.
  await shown(omar, "composer").pressSequentially("Hi");
  await expect(shown(lin, "typing")).toBeVisible({ timeout: 10_000 });
  await sendMessage(omar, "Hi Lin, it works");
  await expect(lin.getByText("Hi Lin, it works", { exact: true }).filter({ visible: true })).toBeVisible({
    timeout: 30_000,
  });
  await expectAccessible(lin);

  await replyAndReact(lin, omar);

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
  await expect(tablet.getByText("Hi Lin, it works", { exact: true }).filter({ visible: true })).toBeVisible({
    timeout: 30_000,
  });
  await Promise.all([...contexts, tabletContext].map((context) => context.close()));
});

/**
 * Ruth changes the open server while Sami watches: a new channel, its
 * place in the list, and a custom emoji in a message.
 */
async function manageTogether(ruth: Page, sami: Page): Promise<void> {
  // Ruth adds a channel in the settings; it shows up for Sami straight away.
  await addChannel(ruth, "announcements");
  await shown(sami, "channel-back").click();
  await openChannel(sami, "announcements");

  // Ruth drags it to the top; Sami's list follows.
  await dragChannelAbove(ruth, "announcements", "general");
  await shown(sami, "channel-back").click();
  await expect.poll(() => channelOrder(sami), { timeout: 30_000 }).toEqual(["announcements", "general"]);

  // Ruth adds a custom emoji and uses it; Sami sees its picture.
  await addEmoji(ruth, "wave");
  await openChannel(ruth, "general");
  await shown(ruth, "composer").fill("Hi :wa");
  await shown(ruth, "emoji-suggestion-wave").click();
  await shown(ruth, "composer-send").click();
  await openChannel(sami, "general");
  await expect(sami.getByRole("img", { name: ":wave:" }).filter({ visible: true }).first()).toBeVisible({
    timeout: 30_000,
  });

  // Sami replies in a thread on Ruth's first message; Ruth sees the count and the reply.
  await replyInThread(sami, "Before you came", "Welcome thread");
  await shown(sami, "thread-back").click();
  await shown(ruth, "channel-back").click();
  await openChannel(ruth, "general");
  await ruth
    .getByRole("button", { name: /^1 reply, last at/ })
    .filter({ visible: true })
    .click({ timeout: 30_000 });
  await expect(ruth.getByText("Welcome thread", { exact: true }).filter({ visible: true })).toBeVisible({
    timeout: 30_000,
  });
  await expectAccessible(ruth);
  await shown(ruth, "thread-back").click();
}

test("a server's channel carries messages, with history for those who join later", async ({ browser }) => {
  test.setTimeout(180_000);
  const contexts = await Promise.all([browser.newContext(), browser.newContext()]);
  const [ruth, sami] = await Promise.all(contexts.map((context) => context.newPage()));
  if (ruth === undefined || sami === undefined) {
    throw new Error("could not open the devices");
  }
  await signUpForMessaging(ruth, accounts.ruth);
  await signUpForMessaging(sami, accounts.sami);
  await makeServer(ruth, "Release club");
  await openChannel(ruth, "general");
  await sendMessage(ruth, "Before you came");
  await shown(ruth, "channel-back").click();
  const invite = await makeInvite(ruth);

  // Sami joins later; Ruth's device sends him the channel's history.
  await joinWith(sami, invite, "Release club");
  await openChannel(sami, "general");
  await expect(sami.getByText("Before you came", { exact: true }).filter({ visible: true })).toBeVisible({
    timeout: 60_000,
  });
  await sendMessage(sami, "Hello from the guest");
  await openChannel(ruth, "general");
  await expect(ruth.getByText("Hello from the guest", { exact: true }).filter({ visible: true })).toBeVisible(
    {
      timeout: 30_000,
    },
  );
  await expectAccessible(sami);

  await shown(ruth, "channel-back").click();
  await manageTogether(ruth, sami);
  await Promise.all(contexts.map((context) => context.close()));
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
