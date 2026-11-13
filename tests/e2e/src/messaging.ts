/**
 * The steps before and of messaging, as a person takes them: writing down
 * the recovery phrase, adding a friend and opening a conversation.
 */
import { expect, type Locator, type Page } from "@playwright/test";

import { expectAccessible } from "./accessibility.js";
import { expectHome, field, fillAccount, type Account } from "./screens.js";

/** The visible element `testId`; screens below the current one stay in the page, hidden. */
export function shown(page: Page, testId: string): Locator {
  return page.getByTestId(testId).filter({ visible: true });
}

/**
 * Reads the phrase shown, says it was written down and passes the check;
 * returns the phrase, as the person would keep it.
 */
export async function writeDownRecoveryPhrase(page: Page): Promise<string> {
  await shown(page, "messaging-next-step").click({ timeout: 60_000 });
  const words = shown(page, "recovery-phrase").getByRole("listitem");
  await expect(words).toHaveCount(24);
  await expectAccessible(page);
  const phrase = (await words.allInnerTexts()).map((text) => text.replace(/^\d+\.\s*/, "").trim());
  await shown(page, "recovery-wrote").click();
  for (let index = 0; ; index++) {
    const input = shown(page, `recovery-word-${String(index)}`);
    if ((await input.count()) === 0) {
      break;
    }
    const label = (await input.getAttribute("aria-label")) ?? "";
    const number = Number(/\d+/.exec(label)?.[0]);
    await input.fill(phrase[number - 1] ?? "");
  }
  await shown(page, "recovery-confirm").click();
  await expect(shown(page, "open-messages")).toBeVisible();
  return phrase.join(" ");
}

/** Restores encryption on another device of the same person. */
export async function restoreWithPhrase(page: Page, phrase: string): Promise<void> {
  await shown(page, "messaging-next-step").click({ timeout: 60_000 });
  await expectAccessible(page);
  await shown(page, "restore-phrase").fill(phrase);
  await shown(page, "restore-submit").click();
  await expect(shown(page, "open-messages")).toBeVisible({ timeout: 60_000 });
}

/** Sends a friend request to `username` from the friends screen. */
export async function requestFriend(page: Page, username: string): Promise<void> {
  await shown(page, "open-friends").click();
  await field(page, "Username").fill(username);
  await shown(page, "friend-send").click();
}

/** Sends `text` from the open conversation. */
export async function sendMessage(page: Page, text: string): Promise<void> {
  const composer = page.getByTestId("composer").filter({ visible: true });
  await composer.fill(text);
  await page.getByTestId("composer-send").filter({ visible: true }).click();
  await expect(composer).toHaveValue("");
}

/** Signs `account` up on a fresh device and writes down its phrase. */
export async function signUpForMessaging(page: Page, account: Account): Promise<string> {
  await page.goto("/");
  await shown(page, "go-register").click();
  await fillAccount(page, account);
  await shown(page, "register-submit").click();
  await expectHome(page, account);
  return writeDownRecoveryPhrase(page);
}

/** `requester` asks `accepter` to be friends, who accepts and opens their DM. */
export async function becomeFriends(
  requester: { page: Page; account: Account },
  accepter: { page: Page; account: Account },
): Promise<void> {
  await requestFriend(requester.page, accepter.account.username);
  await shown(accepter.page, "open-friends").click();
  await shown(accepter.page, `accept-${requester.account.username}`).click();
  await shown(accepter.page, `message-${requester.account.username}`).click();
  await expect(accepter.page.getByText(/Messages here are end-to-end encrypted/)).toBeVisible();
  await expectAccessible(accepter.page);
  await shown(requester.page, `message-${accepter.account.username}`).click();
}

/** Opens the actions of the visible message that says `text`. */
async function messageActions(page: Page, text: string): Promise<void> {
  await page
    .getByLabel(/^Actions for the message from/)
    .filter({ visible: true, hasText: text })
    .click();
}

/** Replies to the message that says `text` with `reply`. */
export async function replyTo(page: Page, text: string, reply: string): Promise<void> {
  await messageActions(page, text);
  await page.getByRole("button", { name: "Reply", exact: true }).filter({ visible: true }).click();
  await expect(page.getByText(/^Replying to/).filter({ visible: true })).toBeVisible();
  await sendMessage(page, reply);
}

/** Reacts to the message that says `text` with `emoji`. */
export async function react(page: Page, text: string, emoji: string): Promise<void> {
  await messageActions(page, text);
  await page
    .getByRole("button", { name: `React with ${emoji}` })
    .filter({ visible: true })
    .click();
}
