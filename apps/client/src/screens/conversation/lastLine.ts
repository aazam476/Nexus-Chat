import type { TFunction } from "i18next";

import { parseInline, plainText } from "../../messaging/markdown";
import type { TimelineItem } from "../../messaging/timeline";

/** The longest preview, in characters; longer text ends in an ellipsis. */
const previewLength = 100;

function shorten(text: string): string {
  const characters = Array.from(text);
  return characters.length <= previewLength ? text : `${characters.slice(0, previewLength - 1).join("")}…`;
}

/** A message as one short plain line, for lists and reply previews. */
export function lastLine(item: TimelineItem, t: TFunction): string {
  if (item.deleted) {
    return t("conversation.deleted");
  }
  const text = plainText(parseInline(item.text)).replace(/\s+/gu, " ").trim();
  if (text !== "") {
    return shorten(text);
  }
  const attachment = item.attachments[0];
  return attachment === undefined ? "" : t("conversation.attachment", { name: attachment.name });
}
