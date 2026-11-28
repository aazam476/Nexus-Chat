/** Filing a report about a message (spec §4.7, ADR 0010). */
import { submitReport } from "@nexus-chat/api-client";
import { useState } from "react";

import { useConnection } from "../../connection/ConnectionContext";
import { toBase64 } from "../../messaging/engine/bytes";
import type { Audience } from "../../messaging/engine/channels";
import type { ReportPlace } from "../../messaging/engine/reports";
import { messengerOf, useMessaging } from "../../messaging/MessagingContext";

/** Where the reported message is, as the screen's link names it. */
export interface ReportTarget {
  messageId: string;
  serverId?: string;
  channelId?: string;
  conversationId?: string;
}

/** How filing went: not yet, sending, sent, or why not. */
export type ReportState =
  { kind: "idle" } | { kind: "sending" } | { kind: "sent" } | { kind: "failed"; code: string };

/** The place of the message for the engine. */
function placeOf(target: ReportTarget, audiences: readonly Audience[]): ReportPlace | undefined {
  if (target.conversationId !== undefined) {
    return { kind: "conversation", conversationId: target.conversationId };
  }
  const channelId = target.channelId;
  const audience = audiences.find(
    (candidate) => channelId !== undefined && candidate.channelIds.includes(channelId),
  );
  return audience === undefined || channelId === undefined
    ? undefined
    : { kind: "channel", audience, channelId };
}

/** The message's place for the API. */
function where(target: ReportTarget): { serverId?: string; channelId?: string; conversationId?: string } {
  return target.conversationId === undefined
    ? { serverId: target.serverId, channelId: target.channelId }
    : { conversationId: target.conversationId };
}

/** Sends the report once the device has made the evidence; `submit` takes the reporter's comment. */
export function useReport(target: ReportTarget): { state: ReportState; submit: (comment: string) => void } {
  const { client } = useConnection();
  const messenger = messengerOf(useMessaging());
  const [state, setState] = useState<ReportState>({ kind: "idle" });
  const submit = (comment: string): void => {
    const place = placeOf(target, messenger?.audienceList() ?? []);
    if (client === undefined || messenger === undefined || place === undefined || state.kind === "sending") {
      return;
    }
    setState({ kind: "sending" });
    void (async () => {
      const evidence = await messenger.reportEvidence(place, target.messageId);
      if (evidence === undefined) {
        setState({ kind: "failed", code: "no_evidence" });
        return;
      }
      const trimmed = comment.trim();
      const body = { ...where(target), messageId: target.messageId, evidence: toBase64(evidence) };
      const answer = await submitReport({
        client,
        body: trimmed === "" ? body : { ...body, comment: trimmed },
      });
      const code = (answer.error as { code?: string } | undefined)?.code ?? "failed";
      setState(answer.response?.ok === true ? { kind: "sent" } : { kind: "failed", code });
    })();
  };
  return { state, submit };
}
