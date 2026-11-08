/**
 * The MLS operations Nexus Chat uses, on top of ts-mls (ADR 0002). Callers
 * see bytes in and bytes out, never library types, so the library can be
 * replaced (OpenMLS is the planned fallback) without touching them.
 *
 * Every function returns the group's new state; the old one must not be
 * used again, because MLS deletes keys as they are used (RFC 9420 §9.2).
 * Key material the library reports as consumed is zeroed straight away.
 */
import {
  acceptAll,
  createApplicationMessage,
  createCommit,
  createGroup as createMlsGroup,
  decodeMlsMessage,
  defaultCapabilities,
  defaultLifetime,
  emptyPskIndex,
  encodeMlsMessage,
  generateKeyPackage,
  joinGroup,
  processMessage,
  zeroOutUint8Array,
  type ClientState,
  type KeyPackage,
  type MLSMessage,
  type PrivateKeyPackage,
  type Proposal,
} from "ts-mls";

import { ciphersuite } from "./ciphersuite.js";

/** One device's MLS identity for joining groups: a key package and its private keys. */
export interface Member {
  /** The key package to publish, as an encoded MLSMessage. */
  keyPackage: Uint8Array;
  publicPackage: KeyPackage;
  secrets: PrivateKeyPackage;
}

/** A device's view of one group. Opaque to callers. */
export interface Group {
  readonly state: ClientState;
}

/** A change to a group, to deliver to its members (and a Welcome to new ones). */
export interface CommitResult {
  group: Group;
  commit: Uint8Array;
  welcome: Uint8Array | undefined;
}

/** Thrown when bytes are not the MLS message the caller expected. */
export class MlsMessageError extends Error {}

function encode(message: MLSMessage): Uint8Array {
  return encodeMlsMessage(message);
}

function decode(bytes: Uint8Array): MLSMessage {
  const decoded = decodeMlsMessage(bytes, 0);
  if (decoded === undefined) {
    throw new MlsMessageError("not an MLS message");
  }
  return decoded[0];
}

/** Creates a device's key package; `identity` is its credential (RFC 9420 §5.3). */
export async function createMember(identity: Uint8Array): Promise<Member> {
  const generated = await generateKeyPackage(
    { credentialType: "basic", identity },
    defaultCapabilities(),
    defaultLifetime,
    [],
    await ciphersuite(),
  );
  const keyPackage = encode({
    version: "mls10",
    wireformat: "mls_key_package",
    keyPackage: generated.publicPackage,
  });
  return { keyPackage, publicPackage: generated.publicPackage, secrets: generated.privatePackage };
}

/** Starts a group with `member` as its only member. */
export async function createGroup(groupId: Uint8Array, member: Member): Promise<Group> {
  return {
    state: await createMlsGroup(groupId, member.publicPackage, member.secrets, [], await ciphersuite()),
  };
}

async function commit(group: Group, proposals: Proposal[]): Promise<CommitResult> {
  const result = await createCommit(
    { state: group.state, cipherSuite: await ciphersuite() },
    { extraProposals: proposals, ratchetTreeExtension: true },
  );
  result.consumed.forEach(zeroOutUint8Array);
  const welcome =
    result.welcome === undefined
      ? undefined
      : encode({ version: "mls10", wireformat: "mls_welcome", welcome: result.welcome });
  return { group: { state: result.newState }, commit: encode(result.commit), welcome };
}

/** Adds the devices whose key packages are given, in one commit. */
export async function addMembers(group: Group, keyPackages: Uint8Array[]): Promise<CommitResult> {
  const proposals: Proposal[] = keyPackages.map((bytes) => {
    const message = decode(bytes);
    if (message.wireformat !== "mls_key_package") {
      throw new MlsMessageError("expected a key package");
    }
    return { proposalType: "add", add: { keyPackage: message.keyPackage } };
  });
  return commit(group, proposals);
}

/** Removes the members at `leafIndices`, in one commit. */
export async function removeMembers(group: Group, leafIndices: number[]): Promise<CommitResult> {
  return commit(
    group,
    leafIndices.map((removed) => ({ proposalType: "remove", remove: { removed } })),
  );
}

/**
 * Refreshes this device's keys with an empty commit carrying an update
 * path, so a compromised past key no longer reads new messages
 * (post-compromise security, RFC 9420 §12.4.2).
 */
export async function updateKeys(group: Group): Promise<CommitResult> {
  return commit(group, []);
}

/** Joins a group from the Welcome a member sent; the ratchet tree travels inside it. */
export async function joinFromWelcome(welcome: Uint8Array, member: Member): Promise<Group> {
  const message = decode(welcome);
  if (message.wireformat !== "mls_welcome") {
    throw new MlsMessageError("expected a welcome");
  }
  const state = await joinGroup(
    message.welcome,
    member.publicPackage,
    member.secrets,
    emptyPskIndex,
    await ciphersuite(),
  );
  return { state };
}

/** What processing a message produced. */
export type Received =
  { kind: "commit"; group: Group } | { kind: "message"; group: Group; plaintext: Uint8Array };

/** Applies a commit or decrypts an application message from another member. */
export async function receive(group: Group, bytes: Uint8Array): Promise<Received> {
  const message = decode(bytes);
  if (message.wireformat !== "mls_private_message" && message.wireformat !== "mls_public_message") {
    throw new MlsMessageError("expected a group message");
  }
  const result = await processMessage(message, group.state, emptyPskIndex, acceptAll, await ciphersuite());
  result.consumed.forEach(zeroOutUint8Array);
  if (result.kind === "applicationMessage") {
    return { kind: "message", group: { state: result.newState }, plaintext: result.message };
  }
  return { kind: "commit", group: { state: result.newState } };
}

/** Encrypts an application message for the group's current epoch. */
export async function encrypt(
  group: Group,
  plaintext: Uint8Array,
): Promise<{ group: Group; message: Uint8Array }> {
  const result = await createApplicationMessage(group.state, plaintext, await ciphersuite());
  result.consumed.forEach(zeroOutUint8Array);
  const message = encode({
    version: "mls10",
    wireformat: "mls_private_message",
    privateMessage: result.privateMessage,
  });
  return { group: { state: result.newState }, message };
}

/** The group's epoch: 0 at creation, one more after every commit. */
export function epoch(group: Group): bigint {
  return group.state.groupContext.epoch;
}

/** The identities of the group's members, by leaf index; removed leaves are absent. */
export function members(group: Group): Map<number, Uint8Array> {
  const result = new Map<number, Uint8Array>();
  group.state.ratchetTree.forEach((node, nodeIndex) => {
    if (node?.nodeType === "leaf" && node.leaf.credential.credentialType === "basic") {
      result.set(nodeIndex / 2, node.leaf.credential.identity);
    }
  });
  return result;
}

/** A value every member derives alike, proving they share the epoch (RFC 9420 §8.7). */
export function epochAuthenticator(group: Group): Uint8Array {
  return group.state.keySchedule.epochAuthenticator;
}
