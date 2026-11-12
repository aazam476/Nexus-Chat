import type { TFunction } from "i18next";

/** A file size in words: bytes, KB, MB or GB, with one decimal above KB. */
export function fileSize(bytes: number, t: TFunction): string {
  if (bytes < 1024) {
    return t("conversation.bytes", { count: bytes });
  }
  if (bytes < 1024 ** 2) {
    return t("conversation.kilobytes", { count: Math.round(bytes / 1024) });
  }
  if (bytes < 1024 ** 3) {
    return t("conversation.megabytes", { count: Math.round((bytes / 1024 ** 2) * 10) / 10 });
  }
  return t("conversation.gigabytes", { count: Math.round((bytes / 1024 ** 3) * 10) / 10 });
}
