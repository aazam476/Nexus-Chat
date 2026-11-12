/**
 * Every screen with demo, worst-case and right-to-left data, in both
 * themes and at phone, 320px (the reflow width of a 1280px window at 400%
 * zoom, which also covers 200% text) and desktop widths (spec §7.2,
 * worst-case UI). Layout checks always run; pixel snapshots run where the
 * rendering is pinned (NEXUS_VISUAL_SNAPSHOTS=1, in the Playwright
 * container).
 */
import { demo, fixedNow, rightToLeft, worstCase, type Dataset } from "@nexus-chat/fixtures";
import { expect, test, type Page } from "@playwright/test";
import { env } from "node:process";

import { expectAccessible } from "../src/accessibility.js";
import { fakeInstance, type FakeInstanceOptions } from "../src/fakeInstance.js";
import { dmId } from "../src/fakeMessaging.js";
import { expectSoundLayout } from "../src/layout.js";
import { sendMessage, shown, writeDownRecoveryPhrase } from "../src/messaging.js";
import { field } from "../src/screens.js";

const snapshots = env.NEXUS_VISUAL_SNAPSHOTS === "1";

/** One way of looking at the app. */
interface Variant {
  dataset: Dataset;
  width: number;
  theme: "light" | "dark";
}

const variants: Variant[] = [
  { dataset: demo, width: 390, theme: "light" },
  { dataset: demo, width: 390, theme: "dark" },
  { dataset: worstCase, width: 320, theme: "light" },
  { dataset: worstCase, width: 390, theme: "light" },
  { dataset: worstCase, width: 390, theme: "dark" },
  { dataset: worstCase, width: 1280, theme: "light" },
  { dataset: rightToLeft, width: 390, theme: "light" },
];

/** A screen and how to get there from a fresh start. */
interface Screen {
  name: string;
  instance?: FakeInstanceOptions;
  open: (page: Page, dataset: Dataset) => Promise<void>;
}

async function logIn(page: Page): Promise<void> {
  await page.goto("/");
  await field(page, "Username or email").fill("someone");
  await field(page, "Password").fill("a long password");
  await page.getByTestId("login-submit").click();
  await expect(page.getByTestId("presence")).toBeVisible();
  // Encryption setup finishes after the screen appears; wait for its
  // result and the last control, so the screen has its final layout.
  await expect(page.getByTestId("messaging-next-step")).toBeVisible();
  await expect(page.getByTestId("logout")).toBeVisible();
}

/** Sets encryption up as a new user would, then opens the conversations. */
async function openConversations(page: Page): Promise<void> {
  await logIn(page);
  await writeDownRecoveryPhrase(page);
  await shown(page, "open-messages").click();
  await expect(shown(page, `conversation-${dmId}`)).toBeVisible();
}

const screens: Screen[] = [
  {
    name: "setup",
    instance: { setupRequired: true },
    open: async (page) => {
      await page.goto("/");
      await page.getByTestId("setup-submit").click();
      await expect(page.getByTestId("setup-submit")).toBeEnabled();
    },
  },
  {
    name: "login",
    open: async (page, dataset) => {
      await page.goto("/");
      await expect(page.getByText(dataset.providers[0]?.displayName ?? "")).toBeVisible();
    },
  },
  {
    name: "register",
    open: async (page, dataset) => {
      await page.goto("/");
      await page.getByTestId("go-register").click();
      await page.getByTestId("register-submit").click();
      await expect(page.getByText(dataset.fieldErrors[0]?.message ?? "")).toBeVisible();
    },
  },
  { name: "home", open: logIn },
  {
    name: "restore",
    open: async (page) => {
      await logIn(page);
      await page.getByTestId("messaging-next-step").click();
      await field(page, "Recovery phrase").fill("not the phrase");
      await page.getByTestId("restore-submit").click();
      await expect(page.getByText(/doesn't open your backup/)).toBeVisible();
    },
  },
  {
    name: "conversations",
    instance: { messaging: true },
    open: openConversations,
  },
  {
    name: "conversation",
    instance: { messaging: true },
    open: async (page, dataset) => {
      await openConversations(page);
      await shown(page, `conversation-${dmId}`).click();
      for (const [index, text] of dataset.messaging.messages.entries()) {
        // A minute apart, so the messages keep their order.
        await page.clock.setFixedTime(Date.parse(fixedNow) + (index + 1) * 60_000);
        await sendMessage(page, text);
      }
      await expect(page.getByTestId(/^message-[0-9a-f-]{36}$/).filter({ visible: true })).toHaveCount(
        dataset.messaging.messages.length,
      );
    },
  },
  {
    name: "admin",
    open: async (page) => {
      await logIn(page);
      await page.getByTestId("go-admin").click();
      await expect(page.getByTestId("smtp-host")).toBeVisible();
      await expect(page.getByTestId("upload-limit")).toBeVisible();
    },
  },
];

/**
 * Grows the window until nothing scrolls: by the hidden height of every
 * scrolling area, since screens scroll inside their own containers (and a
 * conversation's messages inside the screen), so a full-page screenshot
 * would stop at the window.
 */
async function fitToContent(page: Page): Promise<void> {
  const hidden = await page.evaluate(() =>
    Array.from(document.querySelectorAll("body *"))
      .filter((element) => ["auto", "scroll"].includes(getComputedStyle(element).overflowY))
      .reduce((total, element) => total + Math.max(0, element.scrollHeight - element.clientHeight), 0),
  );
  const { width, height } = page.viewportSize() ?? { width: 390, height: 900 };
  await page.setViewportSize({ width, height: Math.max(height + hidden, 900) });
}

for (const variant of variants) {
  const label = `${variant.dataset.name}, ${String(variant.width)}px, ${variant.theme}`;
  test.describe(label, () => {
    test.use({ viewport: { width: variant.width, height: 900 }, colorScheme: variant.theme });

    for (const screen of screens) {
      test(screen.name, async ({ page }) => {
        await page.clock.setFixedTime(fixedNow);
        await fakeInstance(page, variant.dataset, screen.instance);
        await screen.open(page, variant.dataset);

        await fitToContent(page);
        await expectSoundLayout(page);
        await expectAccessible(page);
        if (snapshots) {
          await expect(page).toHaveScreenshot(
            `${screen.name}-${variant.dataset.name}-${String(variant.width)}-${variant.theme}.png`,
            { fullPage: true, animations: "disabled" },
          );
        }
      });
    }
  });
}

test("the layout checks catch clipped text, sideways scrolling and overlapping controls", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 400 });
  await page.setContent(`
    <p style="width: 100px; overflow: hidden; white-space: nowrap">bartholomew.fitzgerald@northwind-industries-holdings.example.com</p>
    <div style="width: 600px">wide</div>
    <button style="position: absolute; top: 200px; left: 10px">One</button>
    <button style="position: absolute; top: 205px; left: 20px">Two</button>
  `);

  const failure = await expectSoundLayout(page).catch((error: unknown) => String(error));

  expect(failure).toContain("text is cut off");
  expect(failure).toContain("the page scrolls sideways");
  expect(failure).toContain("controls overlap");
});
