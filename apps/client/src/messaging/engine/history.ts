/**
 * The decrypted messages of each conversation, kept in the device's
 * encrypted database (spec §5.3). Sync stores what it decrypts in the same
 * pass that moves the conversation's cursor, and a send stores the
 * device's own message, so the conversation reads the same after a
 * restart. Records sort by time, then message ID.
 */
import { utf8 } from "./bytes";
import type { EngineContext } from "./context";
import type { DecryptedMessage } from "./sync";

const collectionOf = (conversationId: string): string => `messages:${conversationId}`;

/** Keeps decrypted messages of a conversation. */
export async function storeMessages(
  context: EngineContext,
  conversationId: string,
  messages: DecryptedMessage[],
): Promise<void> {
  for (const message of messages) {
    await context.database.put(
      collectionOf(conversationId),
      `${message.sentAt}|${message.id}`,
      utf8(JSON.stringify(message)),
    );
  }
}

/** Every stored message of a conversation, oldest first. */
export async function loadMessages(
  context: EngineContext,
  conversationId: string,
): Promise<DecryptedMessage[]> {
  const collection = collectionOf(conversationId);
  const messages: DecryptedMessage[] = [];
  for (const id of await context.database.list(collection)) {
    const stored = await context.database.get(collection, id);
    if (stored !== undefined) {
      messages.push(JSON.parse(new TextDecoder().decode(stored)) as DecryptedMessage);
    }
  }
  return messages;
}
