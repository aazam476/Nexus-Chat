/**
 * Who is typing in a conversation (docs/realtime-protocol.md §4.1). The
 * server keeps no typing state: a member shows as typing for a few seconds
 * after each typing_start, and stops at once when their message arrives.
 */

/** How long a member shows as typing after their last typing_start. */
export const typingShownMs = 6000;
/** How often this device repeats typing while the user keeps typing. */
export const typingRepeatMs = 3000;

/** Each typing member's user ID and when they stop showing, in ms. */
export type Typists = ReadonlyMap<string, number>;

/** `userId` started (or is still) typing at `now`. */
export function startedTyping(typists: Typists, userId: string, now: number): Typists {
  return new Map(typists).set(userId, now + typingShownMs);
}

/** `userId` stopped, for example because their message arrived. */
export function stoppedTyping(typists: Typists, userId: string): Typists {
  if (!typists.has(userId)) {
    return typists;
  }
  const next = new Map(typists);
  next.delete(userId);
  return next;
}

/** The members still typing at `now`, in the order they started. */
export function stillTyping(typists: Typists, now: number): string[] {
  return [...typists].filter(([, until]) => until > now).map(([userId]) => userId);
}

/** Whether to tell the server again, given when this device last did. */
export function shouldReportTyping(lastReported: number | undefined, now: number): boolean {
  return lastReported === undefined || now - lastReported >= typingRepeatMs;
}
