/** A server's invite links (spec §4.4): making, listing and revoking them. */
import {
  createServerInvite,
  listServerInvites,
  revokeServerInvite,
  type Client,
  type ServerInvite,
} from "@nexus-chat/api-client";

import { resultOf, type ServerResult } from "./serverActions";

/** An invite's optional limits: how long it lasts and how often it may be used. */
export interface InviteLimits {
  /** Seconds; undefined for never. */
  expiresInSeconds: number | undefined;
  /** Undefined for no limit. */
  maxUses: number | undefined;
}

/** Makes an invite, or says why not. */
export async function makeInvite(
  client: Client,
  serverId: string,
  limits: InviteLimits,
): Promise<{ ok: true; invite: ServerInvite } | { ok: false; code: string }> {
  const body = {
    ...(limits.expiresInSeconds === undefined ? {} : { expiresInSeconds: limits.expiresInSeconds }),
    ...(limits.maxUses === undefined ? {} : { maxUses: limits.maxUses }),
  };
  const answer = await createServerInvite({ client, path: { serverId }, body });
  if (answer.data !== undefined) {
    return { ok: true, invite: answer.data };
  }
  const result = resultOf(answer);
  return { ok: false, code: result.ok ? "failed" : result.code };
}

/** The server's invites, for those who may manage the server; undefined for anyone else. */
export async function invitesOf(client: Client, serverId: string): Promise<ServerInvite[] | undefined> {
  return (await listServerInvites({ client, path: { serverId } })).data;
}

/** Revokes an invite. */
export async function revokeInvite(client: Client, serverId: string, code: string): Promise<ServerResult> {
  return resultOf(await revokeServerInvite({ client, path: { serverId, code } }), code);
}
