/**
 * Creating, joining and leaving servers, and their invites (spec §4.4), as
 * plain functions over the API client. Each resolves to an ID or the API's
 * error code, which the screens turn into words with `serverErrorText`.
 */
import {
  createServer,
  joinServer,
  leaveServer,
  previewInvite,
  type Client,
  type InvitePreview,
} from "@nexus-chat/api-client";
import type { TFunction } from "i18next";

/** How a server action went: done (with an ID), or the API's error code. */
export type ServerResult = { ok: true; id: string } | { ok: false; code: string };

interface Answer {
  data?: unknown;
  error?: unknown;
  response?: Response | undefined;
}

/** The result of an API call; `id` names what was made, if the answer does not. */
export function resultOf(answer: Answer, id = ""): ServerResult {
  if (answer.response?.ok === true) {
    const data = answer.data as { id?: unknown; serverId?: unknown; server?: { id?: unknown } } | undefined;
    const made = data?.id ?? data?.serverId ?? data?.server?.id;
    return { ok: true, id: typeof made === "string" ? made : id };
  }
  const code = (answer.error as { code?: unknown } | undefined)?.code;
  return { ok: false, code: typeof code === "string" ? code : "failed" };
}

/** Makes a server owned by the user. */
export async function makeServer(client: Client, name: string, description: string): Promise<ServerResult> {
  const trimmed = description.trim();
  const body = trimmed === "" ? { name: name.trim() } : { name: name.trim(), description: trimmed };
  return resultOf(await createServer({ client, body }));
}

/**
 * The invite code in what the user typed or pasted: an invite link
 * (`…/invite/<code>`) or the code itself.
 */
export function inviteCodeOf(text: string): string {
  const trimmed = text.trim();
  const match = /\/invite\/([^/?#\s]+)/.exec(trimmed);
  return (match?.[1] ?? trimmed).toLowerCase();
}

/** What an invite leads to, or undefined when it is not (or no longer) valid. */
export async function lookUpInvite(client: Client, code: string): Promise<InvitePreview | undefined> {
  if (code === "") {
    return undefined;
  }
  const answer = await previewInvite({ client, path: { code } });
  return answer.data;
}

/** Joins the server an invite leads to. */
export async function acceptInvite(client: Client, code: string): Promise<ServerResult> {
  return resultOf(await joinServer({ client, path: { code } }));
}

/** Leaves a server; its owner cannot. */
export async function leave(client: Client, serverId: string): Promise<ServerResult> {
  return resultOf(await leaveServer({ client, path: { serverId } }), serverId);
}

/** The link that opens an invite in the app. */
export function inviteLink(address: string, code: string): string {
  return `${address.replace(/\/+$/, "")}/invite/${code}`;
}

const known = new Set([
  "invalid",
  "invite_invalid",
  "missing_permission",
  "server_not_found",
  "channel_not_found",
  "role_not_found",
  "member_not_found",
  "bad_order",
  "owner_cannot_leave",
  "unsupported_image",
  "image_too_large",
  "invalid_emoji_name",
  "emoji_name_taken",
  "emoji_limit",
  "emoji_not_found",
  "rate_limited",
  "not_banned",
  "banned",
  "invalid_sanction",
  "invalid_timeout",
  "message_not_found",
  "already_handled",
  "not_found",
  "self",
  "last_admin",
  "user_not_found",
  "not_suspended",
]);

/** What to tell the user about a server action's error code. */
export function serverErrorText(code: string, t: TFunction): string {
  return known.has(code) ? t(`servers.errors.${code}`) : t("servers.failed");
}
