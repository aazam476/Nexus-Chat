/**
 * User identity and cross-signing (docs/e2ee-design.md §2): a user master
 * key (UMK) signs a self-signing key (USK), and the USK signs each
 * device's MLS signature key. Everything is Ed25519 from libsodium.
 *
 * Each signed statement starts with its own label and spells out its
 * fields with lengths, so a signature over one kind of statement can never
 * be read as another.
 */
import { sha512 } from "@noble/hashes/sha2.js";
import sodium from "libsodium-wrappers-sumo";

/** An Ed25519 key pair. */
export interface SigningKeyPair {
  publicKey: Uint8Array;
  secretKey: Uint8Array;
}

/** A user's keys right after signup; the master key's secret goes to the backup only. */
export interface NewIdentity {
  masterKey: SigningKeyPair;
  selfSigningKey: SigningKeyPair;
  /** The master key's signature over the self-signing key. */
  selfSigningSignature: Uint8Array;
}

/** What a user publishes on their home instance. */
export interface PublishedIdentity {
  masterPublicKey: Uint8Array;
  selfSigningPublicKey: Uint8Array;
  selfSigningSignature: Uint8Array;
}

/** A device's MLS signature key, as certified by its user. */
export interface DeviceCertificate {
  /** `@user@instance/deviceId`, the device's MLS identity. */
  identity: string;
  signaturePublicKey: Uint8Array;
  signature: Uint8Array;
}

/** Why a certificate chain was refused. */
export class IdentityError extends Error {
  override name = "IdentityError";
}

const selfSigningLabel = "nexus-chat self-signing key v1";
const deviceLabel = "nexus-chat device key v1";
const safetyNumberLabel = "nexus-chat safety number v1";

/** `label`, then each field as a 4-byte big-endian length and its bytes. */
function statement(label: string, ...fields: Uint8Array[]): Uint8Array {
  const parts = [new TextEncoder().encode(label), ...fields];
  const out = new Uint8Array(parts.reduce((total, part) => total + 4 + part.length, 0));
  const view = new DataView(out.buffer);
  let offset = 0;
  for (const part of parts) {
    view.setUint32(offset, part.length);
    out.set(part, offset + 4);
    offset += 4 + part.length;
  }
  return out;
}

const keyPair = (): SigningKeyPair => {
  const { publicKey, privateKey } = sodium.crypto_sign_keypair();
  return { publicKey, secretKey: privateKey };
};

/** Creates a new user's master and self-signing keys. */
export async function createIdentity(): Promise<NewIdentity> {
  await sodium.ready;
  const masterKey = keyPair();
  const selfSigningKey = keyPair();
  return {
    masterKey,
    selfSigningKey,
    selfSigningSignature: sodium.crypto_sign_detached(
      statement(selfSigningLabel, selfSigningKey.publicKey),
      masterKey.secretKey,
    ),
  };
}

/** Replaces the self-signing key, for example after a device holding it was lost. */
export async function rotateSelfSigningKey(
  masterKey: SigningKeyPair,
): Promise<Pick<NewIdentity, "selfSigningKey" | "selfSigningSignature">> {
  await sodium.ready;
  const selfSigningKey = keyPair();
  return {
    selfSigningKey,
    selfSigningSignature: sodium.crypto_sign_detached(
      statement(selfSigningLabel, selfSigningKey.publicKey),
      masterKey.secretKey,
    ),
  };
}

/** Signs a device's MLS signature key with the self-signing key. */
export async function certifyDevice(
  selfSigningKey: SigningKeyPair,
  identity: string,
  signaturePublicKey: Uint8Array,
): Promise<DeviceCertificate> {
  await sodium.ready;
  const signature = sodium.crypto_sign_detached(
    statement(deviceLabel, new TextEncoder().encode(identity), signaturePublicKey),
    selfSigningKey.secretKey,
  );
  return { identity, signaturePublicKey, signature };
}

function verified(signature: Uint8Array, message: Uint8Array, publicKey: Uint8Array): boolean {
  try {
    return sodium.crypto_sign_verify_detached(signature, message, publicKey);
  } catch {
    // Wrong-sized keys or signatures.
    return false;
  }
}

/**
 * Checks that `device` chains to `published`, whose master key the caller
 * has already matched against the one it trusts for this user (trust on
 * first use, or verified in person). Throws an IdentityError if not.
 */
export async function verifyDevice(published: PublishedIdentity, device: DeviceCertificate): Promise<void> {
  await sodium.ready;
  if (
    !verified(
      published.selfSigningSignature,
      statement(selfSigningLabel, published.selfSigningPublicKey),
      published.masterPublicKey,
    )
  ) {
    throw new IdentityError("the self-signing key is not signed by the master key");
  }
  if (
    !verified(
      device.signature,
      statement(deviceLabel, new TextEncoder().encode(device.identity), device.signaturePublicKey),
      published.selfSigningPublicKey,
    )
  ) {
    throw new IdentityError("the device is not signed by its user");
  }
}

/**
 * The master key's safety number: 60 digits in 12 groups of 5, for people
 * to compare in person. Each group is the next 5 bytes of a SHA-512
 * digest, read as a number, mod 100,000.
 */
export function safetyNumber(masterPublicKey: Uint8Array): string {
  const digest = sha512(statement(safetyNumberLabel, masterPublicKey));
  const groups: string[] = [];
  for (let group = 0; group < 12; group++) {
    let value = 0;
    for (const byte of digest.subarray(group * 5, group * 5 + 5)) {
      value = (value * 256 + byte) % 100_000;
    }
    groups.push(String(value).padStart(5, "0"));
  }
  return groups.join(" ");
}

const directiveLabel = "nexus-chat membership directive v1\n";

/**
 * Checks a membership directive's signature against the instance's
 * published key (docs/e2ee-design.md §3.2): the signature covers the label
 * and the directive's exact bytes, as apps/server/internal/delivery signs it.
 */
export async function verifyDirective(
  instanceKey: Uint8Array,
  directive: Uint8Array,
  signature: Uint8Array,
): Promise<boolean> {
  await sodium.ready;
  const message = new Uint8Array([...new TextEncoder().encode(directiveLabel), ...directive]);
  try {
    return sodium.crypto_sign_verify_detached(signature, message, instanceKey);
  } catch {
    return false;
  }
}
