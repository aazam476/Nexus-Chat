/**
 * Typing indicators for one conversation: who else is typing, from the
 * gateway's typing_start events, and reporting the user's own typing.
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { useGatewayEvents, useReportTyping } from "../realtime/GatewayContext";
import { shouldReportTyping, startedTyping, stillTyping, stoppedTyping, type Typists } from "./typing";

/** A typing_start event's data, if it is one for `conversationId`. */
function typistIn(data: unknown, conversationId: string): string | undefined {
  if (typeof data !== "object" || data === null) {
    return undefined;
  }
  const { conversationId: where, userId } = data as { conversationId?: unknown; userId?: unknown };
  return where === conversationId && typeof userId === "string" ? userId : undefined;
}

/** The conversation's latest message: its sender stops showing as typing when it arrives. */
export interface LatestMessage {
  id: string;
  senderId: string | undefined;
}

/** The user IDs typing in the conversation, other than `selfId`. */
export function useTypists(
  conversationId: string,
  selfId: string,
  latest: LatestMessage | undefined,
): string[] {
  const [typists, setTypists] = useState<Typists>(() => new Map());
  const [now, setNow] = useState(() => Date.now());
  const [seen, setSeen] = useState(latest?.id);
  if (latest?.id !== seen) {
    // Adjusting state while rendering, as React recommends for this.
    setSeen(latest?.id);
    if (latest?.senderId !== undefined) {
      setTypists(stoppedTyping(typists, latest.senderId));
    }
  }
  useGatewayEvents((event) => {
    const userId = event.type === "typing_start" ? typistIn(event.data, conversationId) : undefined;
    if (userId !== undefined && userId !== selfId) {
      const at = Date.now();
      setNow(at);
      setTypists((current) => startedTyping(current, userId, at));
    }
  });
  const typing = stillTyping(typists, now);
  const anyone = typing.length > 0;
  useEffect(() => {
    if (!anyone) {
      return;
    }
    // Re-check each second, so an indicator goes away without new events.
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => {
      clearInterval(timer);
    };
  }, [anyone]);
  return typing;
}

/** Call with each change of the draft; reports typing at most every few seconds. */
export function useTypingReporter(conversationId: string): (draft: string) => void {
  const report = useReportTyping();
  const lastRef = useRef<number | undefined>(undefined);
  return useCallback(
    (draft: string) => {
      const now = Date.now();
      if (draft.trim() !== "" && shouldReportTyping(lastRef.current, now)) {
        lastRef.current = now;
        report(conversationId);
      }
    },
    [conversationId, report],
  );
}
