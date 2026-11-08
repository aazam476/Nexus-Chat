/**
 * Theme tokens: every color, size and spacing value the UI uses. Screens
 * never write a color or size by hand; they take it from here, so both
 * themes stay consistent and every pairing is checked for contrast by
 * `tokens.test.ts` (WCAG 2.1 AA, spec §4.12).
 */

/** The color roles of one theme. */
export interface Colors {
  /** Page background. */
  background: string;
  /** Cards and input fields, one step above the background. */
  surface: string;
  /** Body text. */
  text: string;
  /** Secondary text such as hints. */
  textMuted: string;
  /** Primary buttons and links. */
  primary: string;
  /** Text on a primary background. */
  onPrimary: string;
  /** Outlines of inputs and secondary buttons. */
  border: string;
  /** The keyboard focus ring. */
  focus: string;
  /** Errors and destructive actions. */
  danger: string;
  /** Text on a danger background. */
  onDanger: string;
  /** Background of a warning banner. */
  warningSurface: string;
  /** Outline of a warning banner. */
  warningBorder: string;
}

/** Light and dark themes. */
export const colors: Record<"light" | "dark", Colors> = {
  light: {
    background: "#FFFFFF",
    surface: "#F3F4F7",
    text: "#14161B",
    textMuted: "#565C6B",
    primary: "#3A4BD0",
    onPrimary: "#FFFFFF",
    border: "#7D8394",
    focus: "#3A4BD0",
    danger: "#B42318",
    onDanger: "#FFFFFF",
    warningSurface: "#FFF4D6",
    warningBorder: "#9A6A00",
  },
  dark: {
    background: "#0E1014",
    surface: "#1A1D23",
    text: "#ECEEF2",
    textMuted: "#A6ACB9",
    primary: "#8E9AFF",
    onPrimary: "#0E1014",
    border: "#757B8A",
    focus: "#8E9AFF",
    danger: "#FF8A80",
    onDanger: "#0E1014",
    warningSurface: "#2C2206",
    warningBorder: "#B58A26",
  },
};

/** Spacing steps, in points. */
export const spacing = { xs: 4, sm: 8, md: 12, lg: 16, xl: 24, xxl: 32 } as const;

/** Corner radii, in points. */
export const radius = { control: 10, card: 14 } as const;

/** Type sizes and line heights, in points. They grow with the user's font scale. */
export const type = {
  body: { fontSize: 16, lineHeight: 24 },
  small: { fontSize: 14, lineHeight: 20 },
  label: { fontSize: 15, lineHeight: 20, fontWeight: "600" },
  heading1: { fontSize: 28, lineHeight: 34, fontWeight: "700" },
  heading2: { fontSize: 20, lineHeight: 26, fontWeight: "700" },
} as const;

/** The smallest touch target, in points (WCAG 2.5.5, Apple and Material guidelines). */
export const minimumTouchTarget = 44;

/** The widest a single-column form grows on large screens, in points. */
export const formWidth = 480;
