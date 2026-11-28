/**
 * A signed-in device's messaging, without React: it sets up the user's
 * identity and this device's keys, then opens, syncs and sends in
 * conversations and server channels through the engine. Work on one
 * conversation (or one server audience's group) runs one step at a time,
 * since a sync and a send at once would both move the same MLS group.
 * Listeners hear which conversation or channel changed.
 *
 * The messenger remembers which groups are server audiences, on the
 * device, so that a sync after a restart applies them as channels even
 * before the server screens load them again.
 */
import type { LocalDatabase } from "../storage/localDatabase";
import type { MessagingApi } from "./engine/api";
import { syncAudience, type Audience } from "./engine/channels";
import { searchMessages, type SearchFilters, type SearchHit } from "../search/query";
import { backfill, type BackfillProgress } from "./engine/backfill";
import { fetchIndex, shareIndex } from "./engine/indexSync";

/** The queue index sharing runs in, apart from every conversation's. */
const searchQueue = "search-index";
import {
  joinAudience,
  readChannelHistory,
  readChannelHistoryOnce,
  readThreadHistory,
  sendToChannel,
} from "./engine/channelSend";
import { rememberAudience, storedAudience } from "./engine/audienceMemory";
import type { EngineContext } from "./engine/context";
import { readMissedHistory } from "./engine/archive";
import { loadMessages } from "./engine/history";
import {
  checkReport,
  reportEvidence,
  type ReportPlace,
  type ReportToCheck,
  type ReportVerdict,
} from "./engine/reports";
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
  private readonly audiences = new Map<string, Audience>();
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

  /** Applies what is new in the conversation, or in the server audience's group. */
  async sync(conversationId: string): Promise<void> {
    const audience =
      this.audiences.get(conversationId) ?? (await storedAudience(this.context.database, conversationId));
    if (audience !== undefined) {
      return this.syncChannels(audience);
    }
    return this.run(conversationId, async () => {
      await syncConversation(this.context, conversationId);
    });
  }

  /**
   * Makes this device a member of a server audience's group and reads the
   * history of its channels that it has not read yet.
   */
  openAudience(audience: Audience, members: Person[]): Promise<void> {
    this.learn(members);
    this.audiences.set(audience.audienceId, audience);
    return this.run(
      audience.audienceId,
      async () => {
        await rememberAudience(this.context.database, audience);
        await joinAudience(this.context, audience);
        for (const channelId of audience.channelIds) {
          await readChannelHistoryOnce(this.context, audience, channelId);
        }
      },
      audience.channelIds,
    );
  }

  /**
   * Sends a payload in a server channel, or in the thread on `thread`
   * there, and returns its message ID.
   */
  sendInChannel(audience: Audience, channelId: string, payload: Payload, thread?: string): Promise<string> {
    const where = thread === undefined ? { channelId } : { channelId, thread };
    return this.run(audience.audienceId, () => sendToChannel(this.context, audience, where, payload), [
      channelId,
    ]);
  }

  /**
   * Reads the archive of the thread on rootId, for replies this device
   * has not got (sent before it joined, or while its live copies expired).
   */
  readThread(audience: Audience, channelId: string, rootId: string): Promise<void> {
    return this.run(
      audience.audienceId,
      async () => {
        await readThreadHistory(this.context, audience, channelId, rootId);
      },
      [channelId],
    );
  }

  private syncChannels(audience: Audience): Promise<void> {
    return this.run(
      audience.audienceId,
      async () => {
        const { newHistory } = await syncAudience(this.context, audience);
        for (const channelId of newHistory) {
          await readChannelHistory(this.context, audience, channelId);
        }
      },
      audience.channelIds,
    );
  }

  /** Sends a payload and returns its message ID. */
  send(conversationId: string, payload: Payload): Promise<string> {
    return this.run(conversationId, () => sendPayload(this.context, conversationId, payload));
  }

  /** Every message this device holds for the conversation or channel. */
  messages(conversationId: string): Promise<DecryptedMessage[]> {
    return loadMessages(this.context, conversationId);
  }

  /** The server audiences this device keeps. */
  audienceList(): Audience[] {
    return [...this.audiences.values()];
  }

  /**
   * Indexes a conversation's or channel's whole history for search, a
   * page at a time, for as long as `goodTime` says to go on.
   * Call it again to resume.
   */
  searchOlder(id: string, goodTime: () => Promise<boolean>): Promise<BackfillProgress> {
    const audience = this.audienceList().find((candidate) => candidate.channelIds.includes(id));
    const source = audience === undefined ? { conversationId: id } : { channelId: id, audience };
    return this.run(audience?.audienceId ?? id, () => backfill(this.context, source, goodTime), [id]);
  }

  /** Adds the index chunks the user's other devices shared, then shares this device's changes. */
  syncSearchIndex(): Promise<{ fetched: number; shared: number }> {
    return this.run(searchQueue, async () => ({
      fetched: await fetchIndex(this.context),
      shared: await shareIndex(this.context),
    }));
  }

  /** Evidence for reporting a message (ADR 0010), or undefined if this device cannot open its archive copy. */
  reportEvidence(place: ReportPlace, messageId: string): Promise<Uint8Array | undefined> {
    return reportEvidence(this.context, place, messageId);
  }

  /** Checks a report on a moderator's device. */
  checkReport(report: ReportToCheck): Promise<ReportVerdict> {
    return checkReport(this.context, report);
  }

  /** Searches the messages indexed on this device (spec §4.6). */
  search(query: string, filters?: SearchFilters): Promise<SearchHit[]> {
    return searchMessages(this.context.database, query, filters);
  }

  /**
   * Runs `task` after the group's earlier work, then tells listeners about
   * the group and the `channels` it may have changed.
   */
  private run<T>(
    conversationId: string,
    task: () => Promise<T>,
    channels: readonly string[] = [],
  ): Promise<T> {
    const previous = this.queues.get(conversationId) ?? Promise.resolve();
    const next = previous.catch(() => undefined).then(task);
    this.queues.set(conversationId, next);
    return next.finally(() => {
      for (const listener of this.listeners) {
        for (const changed of [conversationId, ...channels]) {
          listener(changed);
        }
      }
    });
  }
}
