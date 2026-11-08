import { contrastRatio, luminance } from "./contrast";
import { colors } from "./tokens";

// WCAG 2.1 AA: 4.5:1 for text, 3:1 for control outlines and focus rings.
const textPairs = [
  ["text", "background"],
  ["text", "surface"],
  ["textMuted", "background"],
  ["textMuted", "surface"],
  ["onPrimary", "primary"],
  ["onDanger", "danger"],
  ["primary", "background"],
  ["danger", "background"],
  ["danger", "surface"],
  ["text", "warningSurface"],
] as const;
const graphicPairs = [
  ["border", "background"],
  ["border", "surface"],
  ["focus", "background"],
  ["focus", "surface"],
  ["warningBorder", "warningSurface"],
  ["primary", "background"],
] as const;

describe.each(["light", "dark"] as const)("the %s theme", (scheme) => {
  const palette = colors[scheme];

  it.each(textPairs)("%s on %s is readable text", (foreground, background) => {
    expect(contrastRatio(palette[foreground], palette[background])).toBeGreaterThanOrEqual(4.5);
  });

  it.each(graphicPairs)("%s against %s is a visible control edge", (foreground, background) => {
    expect(contrastRatio(palette[foreground], palette[background])).toBeGreaterThanOrEqual(3);
  });
});

describe("contrastRatio", () => {
  it("matches the WCAG reference values", () => {
    expect(contrastRatio("#000000", "#FFFFFF")).toBeCloseTo(21, 5);
    expect(contrastRatio("#777777", "#FFFFFF")).toBeCloseTo(4.48, 2);
    expect(contrastRatio("#FFFFFF", "#000000")).toBe(contrastRatio("#000000", "#FFFFFF"));
    expect(luminance("#000000")).toBe(0);
  });

  it("rejects colors that are not #RRGGBB", () => {
    expect(() => luminance("red")).toThrow("not a #RRGGBB color");
  });
});
