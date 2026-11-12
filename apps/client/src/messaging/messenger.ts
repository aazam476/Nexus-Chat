/**
 * A signed-in device's messaging, without React: it sets up the user's
 * identity and this device's keys, then opens, syncs and sends in
 * conversations through the engine. Work on one conversation runs one
 * step at a time, since a sync and a send at once would both move the
 * same MLS group. Listeners hear which conversation changed.
 */
import type { LocalDatabase } from "../storage/localDatabase";
import type { MessagingApi } from "./engine/api";
import { utf8 } from "./engine/bytes";
import type { EngineContext } from "./engine/context";
import { refillKeyPackages, setUpDevice } from "./engine/device";
import { readMissedHistory } from "./engine/archive";
import { loadMessages } from "./engine/history";
import { restoreWithPhrase, setUpIdentity, type StoredIdentity } from "./engine/identity";
import { sendPayload, startConversation } from "./engine/send";
import { syncConversation, type DecryptedMessage } from "./engine/sync";
import { acceptIdentityChange, credentialCheck } from "./engine/trust";
import type { Payload } from "./payload";

/** Someone the messenger may meet in a conversation. */
export interface Person {
  id: string;
  username: string;
}

/** What the messenger needs from the signed-in session. */
export interface MessengerSetup {
  api: MessagingApi;
  database: LocalDatabase;
  user: Person;
  /** The instance's domain and this device's session ID, which make its address. */
  where: { domain: string; deviceId: string };
  /** The instance's public key, which signs membership directives. */
  instanceKey: Uint8Array;
}

/** How starting the messenger went. */
export type MessengerStart =
  { status: "ready"; messenger: Messenger; recoveryPhrase?: string } | { status: "needs-restore" };

type Listener = (conversationId: string) => void;

/** Messaging for one signed-in device. */
export class Messenger {
  private readonly directory = new Map<string, string>();
  private readonly queues = new Map<string, Promise<unknown>>();
  private readonly listeners = new Set<Listener>();
  private readonly changedIdentities = new Set<string>();
  readonly context: EngineContext;

  constructor(
    context: Omit<EngineContext, "check" | "userOfAddress">,
    private readonly domain: string,
  ) {
    const userOfAddress = (user: string): string | undefined => this.directory.get(user);
    const check = credentialCheck(context.api, context.database, userOfAddress, (userId) => {
      this.changedIdentities.add(userId);
    });
    this.context = { ...context, check, userOfAddress };
  }

  /** Remembers people's addresses, so their devices can be checked. */
  learn(people: Person[]): void {
    for (const person of people) {
      this.directory.set(`@${person.username}@${this.domain}`, person.id);
    }
  }

  /** Users whose identity changed since it was pinned; their devices are refused. */
  identityChanges(): string[] {
    return [...this.changedIdentities];
  }

  /** Trusts a user's new identity. */
  async acceptIdentityChange(userId: string): Promise<void> {
    await acceptIdentityChange(this.context.api, this.context.database, userId);
    this.changedIdentities.delete(userId);
  }

  /** Calls `listener` with each conversation that changed; returns the unsubscribe. */
  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  /** Makes this device a member of the conversation, if it is not yet. */
  open(conversationId: string, members: Person[]): Promise<void> {
    this.learn(members);
    const ids = members.map((member) => member.id);
    return this.run(conversationId, async () => {
      await startConversation(this.context, conversationId, ids);
      await readMissedHistory(this.context, conversationId);
    });
  }

  /** Applies what is new in the conversation. */
  sync(conversationId: string): Promise<void> {
    return this.run(conversationId, async () => {
      await syncConversation(this.context, conversationId);
    });
  }

  /** Sends a payload and returns its message ID. */
  send(conversationId: string, payload: Payload): Promise<string> {
    return this.run(conversationId, () => sendPayload(this.context, conversationId, payload));
  }

  /** Every message this device holds for the conversation. */
  messages(conversationId: string): Promise<DecryptedMessage[]> {
    return loadMessages(this.context, conversationId);
  }

  /** Runs `task` after the conversation's earlier work, then tells listeners. */
  private run<T>(conversationId: string, task: () => Promise<T>): Promise<T> {
    const previous = this.queues.get(conversationId) ?? Promise.resolve();
    const next = previous.catch(() => undefined).then(task);
    this.queues.set(conversationId, next);
    return next.finally(() => {
      for (const listener of this.listeners) {
        listener(conversationId);
      }
    });
  }
}

async function ready(setup: MessengerSetup, identity: StoredIdentity): Promise<Messenger> {
  const { api, database, user, where } = setup;
  const device = await setUpDevice(database, identity, {
    user: `@${user.username}@${where.domain}`,
    deviceId: where.deviceId,
  });
  await refillKeyPackages(api, database, device);
  const messenger = new Messenger(
    { api, database, device, instanceKey: setup.instanceKey, backupKey: identity.backupKey },
    where.domain,
  );
  messenger.learn([user]);
  return messenger;
}

const unconfirmed = { collection: "keys", id: "unconfirmed-recovery-phrase" } as const;

/**
 * Sets up messaging on this device, or says the identity must be restored
 * first. A new recovery phrase is kept on the device until the user
 * confirms they wrote it down, so closing the app does not lose it.
 */
export async function startMessenger(setup: MessengerSetup): Promise<MessengerStart> {
  const { database } = setup;
  const result = await setUpIdentity(setup.api, database, setup.user.id);
  if (result.status === "needs-restore") {
    return result;
  }
  if (result.recoveryPhrase !== undefined) {
    await database.put(unconfirmed.collection, unconfirmed.id, utf8(result.recoveryPhrase));
  }
  const messenger = await ready(setup, result.identity);
  const pending = await database.get(unconfirmed.collection, unconfirmed.id);
  return pending === undefined
    ? { status: "ready", messenger }
    : { status: "ready", messenger, recoveryPhrase: new TextDecoder().decode(pending) };
}

/** The user wrote the recovery phrase down: the device forgets it. */
export async function confirmRecoveryPhrase(database: LocalDatabase): Promise<void> {
  await database.delete(unconfirmed.collection, unconfirmed.id);
}

/** Restores the identity from the recovery phrase, then sets up messaging. */
export async function restoreMessenger(setup: MessengerSetup, phrase: string): Promise<Messenger> {
  const identity = await restoreWithPhrase(setup.api, setup.database, setup.user.id, phrase);
  return ready(setup, identity);
}
