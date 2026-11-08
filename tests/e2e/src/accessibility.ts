/**
 * axe-core checks (spec §7.2): every screen the tests visit must have no
 * serious or critical WCAG 2.1 A/AA violations.
 */
import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";

/**
 * Checks the page as it is now. The desktop app's window (`app://`) cannot
 * open the helper page axe normally uses, so there axe runs inside the
 * page itself.
 */
export async function expectAccessible(page: Page): Promise<void> {
  const inPage = page.url().startsWith("app://");
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
    .setLegacyMode(inPage)
    .analyze();
  const blocking = results.violations
    .filter((violation) => violation.impact === "serious" || violation.impact === "critical")
    .map(
      (violation) =>
        `${violation.id}: ${violation.help} (${violation.nodes.map((node) => node.target.join(" ")).join(", ")})`,
    );
  expect(blocking, `accessibility violations on ${page.url()}`).toEqual([]);
}
