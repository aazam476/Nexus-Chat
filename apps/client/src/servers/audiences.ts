/**
 * A server's audiences as the messaging engine needs them
 * (docs/e2ee-design.md §3.1): the text channels this user can see, grouped
 * by the MLS group that carries them, and who of the members may read a
 * channel's history from before they joined.
 */
import type { ServerDetails, ServerMember } from "@nexus-chat/api-client";

import type { Audience } from "../messaging/engine/channels";
import { channelPermissions } from "./permissions";

/** The audiences of the channels in `details`, with `members` to decide history. */
export function audiencesOf(details: ServerDetails, members: readonly ServerMember[]): Audience[] {
  const server = { ownerId: details.server.ownerId, roles: details.roles };
  const mayReadHistory = (userId: string, channelId: string): boolean => {
    const member = members.find((candidate) => candidate.user.id === userId);
    const channel = details.channels.find((candidate) => candidate.id === channelId);
    if (member === undefined || channel === undefined) {
      return false;
    }
    return channelPermissions(server, { userId, roleIds: member.roleIds }, channel.overrides).has(
      "READ_HISTORY_BEFORE_JOIN",
    );
  };
  const channelsByAudience = new Map<string, string[]>();
  for (const channel of details.channels) {
    if (channel.kind === "text" && channel.audienceId !== undefined) {
      channelsByAudience.set(channel.audienceId, [
        ...(channelsByAudience.get(channel.audienceId) ?? []),
        channel.id,
      ]);
    }
  }
  return [...channelsByAudience].map(([audienceId, channelIds]) => ({
    serverId: details.server.id,
    audienceId,
    channelIds,
    mayReadHistory,
  }));
}
