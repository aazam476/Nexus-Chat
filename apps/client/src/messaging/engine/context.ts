/**
 * What every conversation operation needs, and where a conversation's MLS
 * state is kept between launches: its group (saved with packages/crypto)
 * and the ID of the last delivered message this device applied.
 */
import { loadGroup, saveGroup, type CredentialCheck, type Group } from "@nexus-chat/crypto";

import type { LocalDatabase } from "../../storage/localDatabase";
import type { MessagingApi } from "./api";
import { utf8 } from "./bytes";
import type { Device } from "./device";

/** The engine's dependencies for one signed-in device. */
export interface EngineContext {
  api: MessagingApi;
  database: LocalDatabase;
  device: Device;
  check: CredentialCheck;
  /** The instance's public key, which signs membership directives. */
  instanceKey: Uint8Array;
  /** A member's user ID from the `@name@instance` part of their devices' addresses. */
  userOfAddress: (user: string) => string | undefined;
}

/** A conversation's MLS state on this device. */
export interface ConversationState {
  group: Group | undefined;
  /** The last delivered message applied; 0 before any. */
  cursor: number;
}

/** The conversation's saved state, or none yet. */
export async function loadState(context: EngineContext, conversationId: string): Promise<ConversationState> {
  const saved = await context.database.get("groups", conversationId);
  const cursor = await context.database.get("cursors", conversationId);
  return {
    group: saved === undefined ? undefined : loadGroup(saved, context.check),
    cursor: cursor === undefined ? 0 : Number(new TextDecoder().decode(cursor)),
  };
}

/** Saves the conversation's state; a group of undefined forgets it. */
export async function saveState(
  context: EngineContext,
  conversationId: string,
  state: ConversationState,
): Promise<void> {
  if (state.group === undefined) {
    await context.database.delete("groups", conversationId);
  } else {
    await context.database.put("groups", conversationId, saveGroup(state.group));
  }
  await context.database.put("cursors", conversationId, utf8(String(state.cursor)));
}
