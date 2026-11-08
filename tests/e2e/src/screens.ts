/**
 * Steps through the app's screens, as a person would: by visible labels
 * and roles, never by reaching into the app's code.
 */
import { expect, type Locator, type Page } from "@playwright/test";

import { expectAccessible } from "./accessibility.js";

/**
 * The visible text field labelled `label`. Screens below the current one
 * in the navigation stack stay in the page, hidden, so a label alone can
 * match twice.
 */
export function field(page: Page, label: string): Locator {
  return page.getByLabel(label, { exact: true }).filter({ visible: true });
}

/** An account to create or log in with. */
export interface Account {
  username: string;
  displayName: string;
  email: string;
  password: string;
}

/** Fills the four account fields shared by setup and sign-up. */
export async function fillAccount(page: Page, account: Account): Promise<void> {
  await field(page, "Username").fill(account.username);
  await field(page, "Display name").fill(account.displayName);
  await field(page, "Email").fill(account.email);
  await field(page, "Password").fill(account.password);
}

/** Logs in on the login screen and waits for the home screen. */
export async function logIn(page: Page, account: Account): Promise<void> {
  await expect(page.getByTestId("login-submit")).toBeVisible();
  await expectAccessible(page);
  await field(page, "Username or email").fill(account.username);
  await field(page, "Password").fill(account.password);
  await page.getByTestId("login-submit").click();
  await expectHome(page, account);
}

/** Waits for the home screen of `account`, connected over the gateway. */
export async function expectHome(page: Page, account: Account): Promise<void> {
  await expect(page.getByRole("heading", { name: `Welcome, ${account.displayName}` })).toBeVisible();
  await expect(page.getByTestId("presence")).toHaveText("Others see you as: Online");
}
