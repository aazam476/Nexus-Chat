import type { TFunction } from "i18next";

const known = new Set([
  "dm_not_allowed",
  "invalid_group_size",
  "invalid_name",
  "unsupported_image",
  "image_too_large",
  "rate_limited",
]);

/** What to tell the user about a group action's error code. */
export function groupErrorText(code: string, t: TFunction, maximum: number): string {
  return known.has(code) ? t(`groups.errors.${code}`, { maximum }) : t("groups.failed");
}
