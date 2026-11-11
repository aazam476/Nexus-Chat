/**
 * What a conversation is called in lists and headers: a DM by the other
 * person, a group DM by its name or else by its members.
 */
import type { Conversation, UserSummary } from "@nexus-chat/api-client";
import type { TFunction } from "i18next";

import { isolate } from "../i18n/bidi";

/** The members other than the signed-in user. */
export function othersIn(conversation: Conversation, selfId: string): UserSummary[] {
  return conversation.members.filter((member) => member.id !== selfId);
}

/** The conversation's name, with names isolated for right-to-left text. */
export function conversationName(conversation: Conversation, selfId: string, t: TFunction): string {
  if (conversation.name !== undefined && conversation.name !== "") {
    return isolate(conversation.name);
  }
  const [first, ...rest] = othersIn(conversation, selfId);
  if (first === undefined) {
    return "";
  }
  if (rest.length === 0) {
    return isolate(first.displayName);
  }
  return t("conversations.groupName", { first: isolate(first.displayName), count: rest.length });
}
