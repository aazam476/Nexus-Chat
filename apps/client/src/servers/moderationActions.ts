/**
 * Moderation (spec §4.7) as plain functions over the API client: kicking,
 * banning and timing out members, deleting messages, and handling
 * reports. Each resolves to `ServerResult`, whose error codes the screens
 * turn into words with `serverErrorText`.
 */
import {
  banServerMember,
  deleteChannelMessage,
  endServerMemberTimeout,
  escalateServerReport,
  kickServerMember,
  resolveServerReport,
  timeoutServerMember,
  unbanServerMember,
  type Client,
} from "@nexus-chat/api-client";

import { resultOf, type ServerResult } from "./serverActions";

/** A member and the server they are in. */
export interface MemberRef {
  serverId: string;
  userId: string;
}

const reasonOf = (reason: string): { reason?: string } =>
  reason.trim() === "" ? {} : { reason: reason.trim() };

/** Takes a member out of the server; they may join again. */
export async function kick(client: Client, member: MemberRef, reason: string): Promise<ServerResult> {
  return resultOf(await kickServerMember({ client, path: member, body: reasonOf(reason) }), member.userId);
}

/** Bans someone, deleting what they sent in the last `deleteSeconds`. */
export async function ban(
  client: Client,
  member: MemberRef,
  ban: { reason: string; deleteSeconds: number },
): Promise<ServerResult> {
  const body = { ...reasonOf(ban.reason), deleteMessageSeconds: ban.deleteSeconds };
  return resultOf(await banServerMember({ client, path: member, body }), member.userId);
}

/** Lets a banned user join again. */
export async function unban(client: Client, member: MemberRef): Promise<ServerResult> {
  return resultOf(await unbanServerMember({ client, path: member }), member.userId);
}

/** Times a member out for `seconds`, or lifts their timeout when it is 0. */
export async function timeOut(client: Client, member: MemberRef, seconds: number): Promise<ServerResult> {
  const answer =
    seconds === 0
      ? await endServerMemberTimeout({ client, path: member })
      : await timeoutServerMember({ client, path: member, body: { seconds } });
  return resultOf(answer, member.userId);
}

/** Deletes someone's message from a channel, for everyone. */
export async function deleteForEveryone(
  client: Client,
  where: { serverId: string; channelId: string },
  messageId: string,
): Promise<ServerResult> {
  return resultOf(await deleteChannelMessage({ client, path: { ...where, messageId } }), messageId);
}

/** Closes a server's report with a note, or passes it on to the instance admins. */
export async function handleReport(
  client: Client,
  report: { serverId: string; reportId: string },
  outcome: { escalate: true } | { escalate: false; resolution: string },
): Promise<ServerResult> {
  const answer = outcome.escalate
    ? await escalateServerReport({ client, path: report })
    : await resolveServerReport({ client, path: report, body: { resolution: outcome.resolution.trim() } });
  return resultOf(answer, report.reportId);
}
