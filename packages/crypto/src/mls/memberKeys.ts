/**
 * What the archive copy of a message (docs/e2ee-design.md §4) needs from
 * the MLS group: secrets exported from the current epoch, and this
 * device's MLS signature key, with which it signs what it sends.
 * Signatures use MLS's labelled signing (RFC 9420 §5.1.2), so they can
 * never be mistaken for signatures MLS itself makes.
 */
import { mlsExporter } from "ts-mls";
import { signWithLabel, verifyWithLabel } from "ts-mls/crypto/signature.js";

import { ciphersuite } from "./ciphersuite.js";
import type { Group } from "./engine.js";

/** RFC 9420 §8.5: a secret for `label` and `context`, from the group's current epoch. */
export async function exportSecret(
  group: Group,
  label: string,
  context: Uint8Array,
  length: number,
): Promise<Uint8Array> {
  return mlsExporter(group.state.keySchedule.exporterSecret, label, context, length, await ciphersuite());
}

/** This device's leaf: its credential's identity bytes and its MLS signature public key. */
export function ownLeaf(group: Group): { identity: Uint8Array; signaturePublicKey: Uint8Array } {
  const node = group.state.ratchetTree[group.state.privatePath.leafIndex * 2];
  if (node?.nodeType !== "leaf" || node.leaf.credential.credentialType !== "basic") {
    throw new Error("this device has no basic-credential leaf in the group");
  }
  return { identity: node.leaf.credential.identity, signaturePublicKey: node.leaf.signaturePublicKey };
}

/** Signs `content` with this device's MLS signature key under `label`. */
export async function signAsMember(group: Group, label: string, content: Uint8Array): Promise<Uint8Array> {
  return signWithLabel(group.state.signaturePrivateKey, label, content, (await ciphersuite()).signature);
}

/** Checks a signature made with `signAsMember`. */
export async function verifyMemberSignature(
  signaturePublicKey: Uint8Array,
  label: string,
  content: Uint8Array,
  signature: Uint8Array,
): Promise<boolean> {
  try {
    return await verifyWithLabel(
      signaturePublicKey,
      label,
      content,
      signature,
      (await ciphersuite()).signature,
    );
  } catch {
    // A malformed key or signature.
    return false;
  }
}
