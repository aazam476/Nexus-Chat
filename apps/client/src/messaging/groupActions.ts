/**
 * Creating and changing group DMs (spec §4.3), as plain functions over the
 * API client. Each resolves to the conversation ID or an error code the
 * screens turn into words.
 */
import {
  addConversationMember,
  createGroupConversation,
  leaveConversation,
  removeConversationIcon,
  renameConversation,
  setConversationIcon,
  type Client,
} from "@nexus-chat/api-client";

/** How a group action went: done, or the API's error code. */
export type GroupResult = { ok: true; id: string } | { ok: false; code: string };

interface Answer {
  data?: unknown;
  error?: unknown;
  response?: Response | undefined;
}

function resultOf(answer: Answer, id: string): GroupResult {
  if (answer.response?.ok === true) {
    const created = (answer.data as { id?: unknown } | undefined)?.id;
    return { ok: true, id: typeof created === "string" ? created : id };
  }
  const code = (answer.error as { code?: unknown } | undefined)?.code;
  return { ok: false, code: typeof code === "string" ? code : "failed" };
}

/** Creates a group DM with the caller and `memberIds`. */
export async function createGroup(client: Client, memberIds: string[], name: string): Promise<GroupResult> {
  const trimmed = name.trim();
  const body = trimmed === "" ? { memberIds } : { memberIds, name: trimmed };
  return resultOf(await createGroupConversation({ client, body }), "");
}

/** Renames a group DM; an empty name removes it. */
export async function renameGroup(
  client: Client,
  conversationId: string,
  name: string,
): Promise<GroupResult> {
  const trimmed = name.trim();
  const body = trimmed === "" ? {} : { name: trimmed };
  return resultOf(await renameConversation({ client, path: { conversationId }, body }), conversationId);
}

/** Adds a member to a group DM. */
export async function addToGroup(
  client: Client,
  conversationId: string,
  userId: string,
): Promise<GroupResult> {
  return resultOf(
    await addConversationMember({ client, path: { conversationId }, body: { userId } }),
    conversationId,
  );
}

/** Leaves a group DM. */
export async function leaveGroup(client: Client, conversationId: string): Promise<GroupResult> {
  return resultOf(await leaveConversation({ client, path: { conversationId } }), conversationId);
}

/** Uploads an image as the group's icon. */
export async function setGroupIcon(
  client: Client,
  conversationId: string,
  image: Blob,
): Promise<GroupResult> {
  return resultOf(
    await setConversationIcon({ client, path: { conversationId }, body: image }),
    conversationId,
  );
}

/** Removes the group's icon. */
export async function removeGroupIcon(client: Client, conversationId: string): Promise<GroupResult> {
  return resultOf(await removeConversationIcon({ client, path: { conversationId } }), conversationId);
}
