/**
 * Turns the stream of decrypted messages into what a conversation shows
 * (spec §4.5). Edits, deletions and reactions are messages too; they
 * change the message they name. Only the original sender's user may edit
 * or delete it, so a member cannot rewrite someone else's words; anyone in
 * the conversation may react.
 */
import { userOf } from "@nexus-chat/crypto";

import type { AttachmentReference, LinkPreview } from "./payload";
import type { DecryptedMessage } from "./engine/sync";

/** One message as the conversation shows it. */
export interface TimelineItem {
  id: string;
  /** `@name@instance`, the sender's user. */
  sender: string;
  sentAt: string;
  text: string;
  edited: boolean;
  deleted: boolean;
  replyTo?: string;
  attachments: AttachmentReference[];
  preview?: LinkPreview;
  /** The live copy's ID on the delivery service, for read state; missing on old messages. */
  deliveryId?: number;
  /** Each emoji with the users who reacted with it, in the order they did. */
  reactions: { emoji: string; users: string[] }[];
  /** For a reply in a thread, the message the thread is on. */
  thread?: string;
  /** On a message with a thread: how many replies, and when the last came. */
  replies?: { count: number; lastAt: string };
}

function userOfSender(sender: string): string {
  return userOf(sender) ?? sender;
}

function react(item: TimelineItem, emoji: string, user: string, added: boolean): void {
  const existing = item.reactions.find((reaction) => reaction.emoji === emoji);
  if (added && existing === undefined) {
    item.reactions.push({ emoji, users: [user] });
  } else if (added && existing !== undefined && !existing.users.includes(user)) {
    existing.users.push(user);
  } else if (!added && existing !== undefined) {
    existing.users = existing.users.filter((reactor) => reactor !== user);
    item.reactions = item.reactions.filter((reaction) => reaction.users.length > 0);
  }
}

/** Applies an edit, deletion or reaction to the message it names, if allowed. */
function change(items: Map<string, TimelineItem>, message: DecryptedMessage): void {
  const { payload } = message;
  if (payload.kind === "text") {
    return;
  }
  const target = items.get(payload.target);
  const user = userOfSender(message.sender);
  if (target === undefined || target.deleted) {
    return;
  }
  if (payload.kind === "reaction") {
    react(target, payload.emoji, user, payload.added);
  } else if (target.sender === user && payload.kind === "edit") {
    target.text = payload.text;
    target.edited = true;
  } else if (target.sender === user && payload.kind === "delete") {
    Object.assign(target, { text: "", deleted: true, attachments: [], reactions: [], preview: undefined });
  }
}

/** The conversation's messages in the order they were sent, with every change applied. */
export function buildTimeline(messages: DecryptedMessage[]): TimelineItem[] {
  const ordered = [...messages].sort((first, second) => first.sentAt.localeCompare(second.sentAt));
  const items = new Map<string, TimelineItem>();
  for (const message of ordered) {
    const { payload } = message;
    if (payload.kind === "text" && !items.has(message.id)) {
      items.set(message.id, {
        id: message.id,
        sender: userOfSender(message.sender),
        sentAt: message.sentAt,
        text: payload.text,
        edited: false,
        deleted: false,
        ...(payload.replyTo === undefined ? {} : { replyTo: payload.replyTo }),
        attachments: payload.attachments ?? [],
        ...(payload.preview === undefined ? {} : { preview: payload.preview }),
        ...(message.deliveryId === undefined ? {} : { deliveryId: message.deliveryId }),
        ...(message.thread === undefined ? {} : { thread: message.thread }),
        reactions: [],
      });
    } else {
      change(items, message);
    }
  }
  return [...items.values()];
}

/**
 * What a channel shows: its own messages, without replies in threads, each
 * with how many replies its thread has.
 */
export function channelItems(timeline: TimelineItem[]): TimelineItem[] {
  const replies = new Map<string, { count: number; lastAt: string }>();
  for (const item of timeline) {
    if (item.thread !== undefined) {
      const known = replies.get(item.thread);
      replies.set(item.thread, { count: (known?.count ?? 0) + 1, lastAt: item.sentAt });
    }
  }
  return timeline
    .filter((item) => item.thread === undefined)
    .map((item) => {
      const thread = replies.get(item.id);
      return thread === undefined ? item : { ...item, replies: thread };
    });
}

/** What a thread shows: the message it is on (once this device has it), then its replies. */
export function threadItems(timeline: TimelineItem[], rootId: string): TimelineItem[] {
  return timeline.filter((item) => item.id === rootId || item.thread === rootId);
}

/** The highest delivery ID among the messages, edits and reactions included; 0 without any. */
export function newestDeliveryId(messages: DecryptedMessage[]): number {
  return messages.reduce((newest, message) => Math.max(newest, message.deliveryId ?? 0), 0);
}

/**
 * The first message from someone else after `lastReadId`: where "new
 * messages" starts. Undefined when everything was read.
 */
export function firstUnread(
  timeline: TimelineItem[],
  lastReadId: number,
  self: string,
): TimelineItem | undefined {
  return timeline.find((item) => item.sender !== self && (item.deliveryId ?? 0) > lastReadId);
}
