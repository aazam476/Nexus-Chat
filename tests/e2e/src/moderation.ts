/** Steps for moderating a server and the instance in the browser tests (spec §4.7). */
import { expect, type Page } from "@playwright/test";

import { expectAccessible } from "./accessibility.js";
import { messageActions, shown } from "./messaging.js";

/** Reports the visible message that says `text`, with a comment. */
export async function reportMessage(page: Page, text: string, comment: string): Promise<void> {
  await messageActions(page, text);
  await shown(page, "message-report").click();
  await shown(page, "report-comment").fill(comment);
  await expectAccessible(page);
  await shown(page, "report-send").click();
  await expect(shown(page, "report-sent")).toBeVisible({ timeout: 30_000 });
  await shown(page, "report-back").click();
}

/** From the open server, resolves its one open report after seeing it verified on this device. */
export async function resolveReport(page: Page, sender: string, note: string): Promise<void> {
  await shown(page, "server-settings").click();
  await shown(page, "manage-reports").click();
  await expect(
    page.getByText(new RegExp(`^Verified: sent by .?@${sender}@`)).filter({ visible: true }),
  ).toBeVisible({
    timeout: 30_000,
  });
  await expectAccessible(page);
  await page.getByLabel("What was done").filter({ visible: true }).fill(note);
  await page.getByRole("button", { name: "Resolve", exact: true }).filter({ visible: true }).click();
  await expect(page.getByText("No reports here.").filter({ visible: true })).toBeVisible();
  await shown(page, "reports-back").click();
}

/** From the server's settings, kicks the member called `name`, then reads it in the audit log. */
export async function kickMember(
  page: Page,
  member: { name: string; username: string },
  reason: string,
): Promise<void> {
  const { name, username } = member;
  await shown(page, "manage-members").click();
  await page
    .getByRole("button", { name: new RegExp(name) })
    .filter({ visible: true })
    .first()
    .click();
  await shown(page, "member-reason").fill(reason);
  await expectAccessible(page);
  await shown(page, "member-kick").click();
  await shown(page, "member-kick-confirm").click();
  await expect(page.getByText(new RegExp(name)).filter({ visible: true })).toHaveCount(0, {
    timeout: 30_000,
  });
  await shown(page, "members-back").click();
  await shown(page, "manage-audit-log").click();
  await expect(page.getByText(new RegExp(`kicked @.?${username}`)).filter({ visible: true })).toBeVisible();
  await expect(page.getByText(new RegExp(`Reason: .?${reason}`)).filter({ visible: true })).toBeVisible();
  await expectAccessible(page);
  await shown(page, "audit-back").click();
}

/** From the Admin panel, suspends or reinstates the account `username`. */
export async function setSuspended(page: Page, username: string, suspend: boolean): Promise<void> {
  await shown(page, "admin-users").click();
  await shown(page, "admin-users-search").fill(username);
  const row = page.getByText(new RegExp(`^@.?${username}.? ·`)).filter({ visible: true });
  await expect(row).toBeVisible();
  await expectAccessible(page);
  const button = page
    .getByRole("button", { name: suspend ? "Suspend" : "Reinstate", exact: true })
    .filter({ visible: true });
  await button.click();
  await expect(page.getByText(/^Suspended:/).filter({ visible: true })).toHaveCount(suspend ? 1 : 0);
  await shown(page, "admin-page-back").click();
}
