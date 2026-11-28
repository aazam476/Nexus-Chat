/** How an audit entry reads: who did what to what, in the user's words. */
import type { AuditEntry, ServerMember } from "@nexus-chat/api-client";
import type { TFunction } from "i18next";

import { isolate } from "../../../i18n/bidi";

/** The actions the app has words for; others read as their raw name. */
const known = new Set([
  "server.update",
  "server.icon",
  "channel.create",
  "channel.update",
  "channel.delete",
  "channel.order",
  "channel.override_set",
  "channel.override_clear",
  "role.create",
  "role.update",
  "role.delete",
  "role.order",
  "member.role_give",
  "member.role_take",
  "member.kick",
  "member.ban",
  "member.unban",
  "member.timeout",
  "invite.create",
  "invite.revoke",
  "emoji.add",
  "emoji.remove",
  "message.delete",
  "report.resolve",
  "report.escalate",
]);

/** A member's name, or a stand-in for someone no longer known. */
export function personName(
  members: readonly ServerMember[],
  userId: string | undefined,
  t: TFunction,
): string {
  const member = members.find((candidate) => candidate.user.id === userId);
  return member === undefined ? t("moderation.audit.someone") : isolate(member.user.displayName);
}

/** What the entry's target was called, from its details or the members list. */
function targetName(entry: AuditEntry, members: readonly ServerMember[], t: TFunction): string {
  const named = entry.details.name ?? entry.details.role;
  if (typeof named === "string") {
    return isolate(named);
  }
  if (entry.targetKind === "member") {
    const username = entry.details.username;
    const known = members.some((member) => member.user.id === entry.targetId);
    return !known && typeof username === "string"
      ? `@${isolate(username)}`
      : personName(members, entry.targetId, t);
  }
  return entry.targetId === undefined ? "" : isolate(entry.targetId);
}

/** One line for an audit entry. */
export function auditLine(entry: AuditEntry, members: readonly ServerMember[], t: TFunction): string {
  const actor = personName(members, entry.actorId, t);
  const target = targetName(entry, members, t);
  if (!known.has(entry.action)) {
    return t("moderation.audit.other", { actor, action: entry.action, target });
  }
  return t(`moderation.audit.actions.${entry.action.replace(".", "_")}`, { actor, target });
}

/** The reason an entry records, if any. */
export function auditReason(entry: AuditEntry): string | undefined {
  const reason = entry.details.reason;
  return typeof reason === "string" && reason !== "" ? reason : undefined;
}
