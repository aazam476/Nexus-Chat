import type { TFunction } from "i18next";

import { parseInline, plainText } from "../../messaging/markdown";
import type { TimelineItem } from "../../messaging/timeline";

/** A message as one plain line, for lists and reply previews. */
export function lastLine(item: TimelineItem, t: TFunction): string {
  if (item.deleted) {
    return t("conversation.deleted");
  }
  const text = plainText(parseInline(item.text)).replace(/\s+/gu, " ").trim();
  if (text !== "") {
    return text;
  }
  const attachment = item.attachments[0];
  return attachment === undefined ? "" : t("conversation.attachment", { name: attachment.name });
}
