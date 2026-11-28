/**
 * Reports (spec §4.7, ADR 0010). A reporter's device opens the reported
 * message's archive copy and hands over what it proves: the plaintext and
 * its sender's signature over it, at its place in its conversation or
 * channel. The instance adds a signed receipt for the copy it stores. A
 * moderator's device then checks, without any history key, that the
 * instance signed the receipt, that the evidence is about the message the
 * receipt names, that the sender signed exactly this plaintext, and that
 * the sender's device belongs to the user the receipt names. A forged or
 * altered message, or someone else's words passed off as the sender's,
 * fails one of those checks.
 */
import {
  archiveEvidence,
  decodeDeviceCredential,
  decodeEvidence,
  encodeEvidence,
  userOf,
  verifyEvidence,
  verifyReceipt,
  type ArchiveEvidence,
} from "@nexus-chat/crypto";

import { decodePayload, type Payload } from "../payload";
import { backedUpKeys, keptHistoryKeys } from "./archive";
import { fromBase64, utf8 } from "./bytes";
import { channelKeysId, keyName, type Audience } from "./channels";
import type { EngineContext } from "./context";

/** Where a reported message is: a server channel, or a direct or group conversation. */
export type ReportPlace =
  | { kind: "channel"; audience: Audience; channelId: string }
  | { kind: "conversation"; conversationId: string };

/** The ID the message's archive copy is addressed by. */
const placeId = (place: ReportPlace): string =>
  place.kind === "channel" ? place.channelId : place.conversationId;

/** The copy and the history key that opens it, if this device has both. */
async function sealedCopy(
  context: EngineContext,
  place: ReportPlace,
  messageId: string,
): Promise<{ epoch: number; sealed: Uint8Array; key?: string } | undefined> {
  if (place.kind === "channel") {
    const copy = await context.api.channelArchivedCopy(
      { serverId: place.audience.serverId, channelId: place.channelId },
      messageId,
    );
    const keys = await keptHistoryKeys(context, channelKeysId(place.channelId));
    return copy === undefined ? undefined : { ...copy, key: keys[keyName(copy.groupId, copy.epoch)] };
  }
  const copy = await context.api.archivedCopy(place.conversationId, messageId);
  const keys = {
    ...(await backedUpKeys(context, place.conversationId)),
    ...(await keptHistoryKeys(context, place.conversationId)),
  };
  return copy === undefined ? undefined : { ...copy, key: keys[String(copy.epoch)] };
}

/** Evidence for reporting a message, or undefined when this device cannot open its archive copy. */
export async function reportEvidence(
  context: EngineContext,
  place: ReportPlace,
  messageId: string,
): Promise<Uint8Array | undefined> {
  const copy = await sealedCopy(context, place, messageId);
  if (copy?.key === undefined) {
    return undefined;
  }
  try {
    const address = { channelId: utf8(placeId(place)), messageId: utf8(messageId) };
    const evidence = await archiveEvidence(
      { epoch: BigInt(copy.epoch), sealed: copy.sealed },
      address,
      fromBase64(copy.key),
    );
    return encodeEvidence(evidence);
  } catch {
    return undefined;
  }
}

/** A report as a moderator's device receives it. */
export interface ReportToCheck {
  messageId: string;
  evidence: Uint8Array;
  receipt: Uint8Array;
  receiptSignature: Uint8Array;
}

/** What the instance's receipt says, as apps/server/internal/reports writes it. */
interface Receipt {
  conversationId?: string;
  channelId?: string;
  messageId?: string;
  sender?: string;
  epoch?: number;
  sentAt?: string;
}

/** Why a report did not check out. */
export type ReportProblem = "receipt" | "evidence" | "mismatch" | "signature" | "sender";

/** A moderator device's verdict on a report. */
export type ReportVerdict =
  | { verified: true; sender: string; sentAt: string; payload: Payload }
  | { verified: false; problem: ReportProblem };

const text = (bytes: Uint8Array): string => new TextDecoder().decode(bytes);

/** Whether the evidence is about the message the receipt and report name. */
function about(evidence: ArchiveEvidence, receipt: Receipt, messageId: string): boolean {
  return (
    text(evidence.address.channelId) === (receipt.channelId ?? receipt.conversationId) &&
    text(evidence.address.messageId) === messageId &&
    receipt.messageId === messageId &&
    Number(evidence.epoch) === receipt.epoch
  );
}

/** The receipt, if the instance signed it. */
async function signedReceipt(context: EngineContext, report: ReportToCheck): Promise<Receipt | undefined> {
  if (!(await verifyReceipt(context.instanceKey, report.receipt, report.receiptSignature))) {
    return undefined;
  }
  try {
    return JSON.parse(text(report.receipt)) as Receipt;
  } catch {
    return undefined;
  }
}

/** Checks a report on a moderator's device; nothing it says is taken on trust. */
export async function checkReport(context: EngineContext, report: ReportToCheck): Promise<ReportVerdict> {
  const receipt = await signedReceipt(context, report);
  if (receipt === undefined) {
    return { verified: false, problem: "receipt" };
  }
  const evidence = decodeEvidence(report.evidence);
  const envelope = evidence === undefined ? undefined : decodePayload(evidence.plaintext);
  if (evidence === undefined || envelope === undefined) {
    return { verified: false, problem: "evidence" };
  }
  if (!about(evidence, receipt, report.messageId) || envelope.id !== report.messageId) {
    return { verified: false, problem: "mismatch" };
  }
  if (!(await verifyEvidence(evidence, () => Promise.resolve(true)))) {
    return { verified: false, problem: "signature" };
  }
  const device = decodeDeviceCredential(evidence.sender)?.identity;
  const sender = device === undefined ? undefined : userOf(device);
  if (
    sender === undefined ||
    sender !== receipt.sender ||
    !(await context.check(evidence.sender, evidence.signaturePublicKey))
  ) {
    return { verified: false, problem: "sender" };
  }
  return { verified: true, sender, sentAt: receipt.sentAt ?? "", payload: envelope.payload };
}
