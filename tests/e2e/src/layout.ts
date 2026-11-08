/**
 * Layout checks that hold on every screen with any data (spec §7.2: no
 * clipped text, overlapping elements, or broken layouts). They run in any
 * browser, unlike pixel snapshots, which need the pinned container.
 */
import { expect, type Page } from "@playwright/test";

/** Text that is cut off or runs past the screen, and sideways scrolling. These run in the page. */
function overflowProblems(): string[] {
  const problems: string[] = [];
  if (document.documentElement.scrollWidth > window.innerWidth + 1) {
    problems.push(`the page scrolls sideways (${String(document.documentElement.scrollWidth)}px wide)`);
  }
  for (const element of Array.from(document.querySelectorAll("body *"))) {
    const hasText = Array.from(element.childNodes).some(
      (node) => node.nodeType === Node.TEXT_NODE && (node.textContent ?? "").trim() !== "",
    );
    if (!hasText) {
      continue;
    }
    const style = getComputedStyle(element);
    const clips = style.overflowX !== "visible" || style.textOverflow === "ellipsis";
    const name = `${element.tagName.toLowerCase()} "${element.textContent.trim().slice(0, 40)}"`;
    if (clips && element.scrollWidth > element.clientWidth + 1) {
      problems.push(`text is cut off: ${name}`);
    }
    if (element.getBoundingClientRect().right > window.innerWidth + 1) {
      problems.push(`text runs off the screen: ${name}`);
    }
  }
  return problems;
}

/** Controls drawn on top of each other. */
function overlapProblems(): string[] {
  const problems: string[] = [];
  const controls = Array.from(document.querySelectorAll("button, input, textarea, a, [role=button]")).filter(
    (control) => control.getBoundingClientRect().width > 0,
  );
  controls.forEach((first, index) => {
    for (const second of controls.slice(index + 1)) {
      const a = first.getBoundingClientRect();
      const b = second.getBoundingClientRect();
      const across = Math.min(a.right, b.right) - Math.max(a.left, b.left);
      const down = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
      if (!first.contains(second) && !second.contains(first) && across > 1 && down > 1) {
        problems.push(`controls overlap: "${first.textContent.trim()}" and "${second.textContent.trim()}"`);
      }
    }
  });
  return problems;
}

/** Fails with every layout problem on the page. */
export async function expectSoundLayout(page: Page): Promise<void> {
  const problems = [...(await page.evaluate(overflowProblems)), ...(await page.evaluate(overlapProblems))];
  expect(problems, `layout problems on ${page.url()}`).toEqual([]);
}
