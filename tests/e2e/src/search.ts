/** Searching messages on the device, as a person does: from a conversation's "Search here". */
import { expect, type Page } from "@playwright/test";

import { expectAccessible } from "./accessibility.js";
import { shown } from "./messaging.js";

/** Searches the open conversation for `query` and checks `expected` is among the results. */
export async function searchHere(page: Page, query: string, expected: string): Promise<void> {
  await shown(page, "search-here").click();
  await shown(page, "search-query").fill(query);
  await expect(page.getByText(expected, { exact: true }).filter({ visible: true }).last()).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByText(/^\d+ messages?$/).filter({ visible: true })).toBeVisible();
  await expectAccessible(page);
  await shown(page, "search-back").click();
}
