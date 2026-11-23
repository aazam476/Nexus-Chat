/** The steps of servers, as a person takes them: making one, inviting, joining, opening a channel. */
import { expect, type Page } from "@playwright/test";

import { expectAccessible } from "./accessibility.js";
import { shown } from "./messaging.js";

/** Makes a server from the home screen and leaves its screen open. */
export async function makeServer(page: Page, name: string): Promise<void> {
  await shown(page, "open-servers").click();
  await shown(page, "server-new").click();
  await shown(page, "server-name").fill(name);
  await shown(page, "server-create").click();
  await expect(page.getByRole("heading", { name }).filter({ visible: true })).toBeVisible();
  await expectAccessible(page);
}

/** Makes an invite link from the open server and returns its path. */
export async function makeInvite(page: Page): Promise<string> {
  await shown(page, "server-invites").click();
  await shown(page, "invite-create").click();
  const link = await shown(page, "invite-link").innerText({ timeout: 30_000 });
  await expectAccessible(page);
  await shown(page, "invites-back").click();
  return new URL(link).pathname;
}

/** Opens an invite link and joins its server. */
export async function joinWith(page: Page, invitePath: string, serverName: string): Promise<void> {
  await page.goto(invitePath);
  await shown(page, "invite-join").click({ timeout: 30_000 });
  await expect(page.getByRole("heading", { name: serverName }).filter({ visible: true })).toBeVisible();
}

/** A channel's name as the app shows it: `#`, then the name isolated for right-to-left text. */
function channelTitle(name: string): RegExp {
  return new RegExp(`^#\u2068?${name}\u2069?$`);
}

/** A channel row in a server's list: its title, then an unread count when there is one. */
function channelRow(name: string): RegExp {
  return new RegExp(`^#\u2068?${name}\u2069?(?![\\w-])`);
}

/** Opens the text channel `name` of the open server. */
export async function openChannel(page: Page, name: string): Promise<void> {
  await page
    .getByRole("button", { name: channelRow(name) })
    .filter({ visible: true })
    .first()
    .click();
  await expect(
    page.getByRole("heading", { name: channelTitle(name) }).filter({ visible: true }),
  ).toBeVisible();
}

/** Adds a text channel from the open server's settings, then goes back to the server. */
export async function addChannel(page: Page, name: string): Promise<void> {
  await shown(page, "server-settings").click();
  await shown(page, "manage-channels").click();
  await shown(page, "new-channel-name").fill(name);
  await shown(page, "new-channel-create").click();
  await expect(page.getByText(channelTitle(name)).filter({ visible: true })).toBeVisible();
  await expectAccessible(page);
  await shown(page, "channel-admin-back").click();
  await shown(page, "settings-back").click();
}
