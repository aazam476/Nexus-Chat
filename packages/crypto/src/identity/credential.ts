/**
 * The identity a device puts in its MLS BasicCredential, and the check
 * every member runs on every other member (docs/e2ee-design.md §2, RFC 9420
 * §5.3.1). The credential holds the device's address and its user's
 * certificate over the device's MLS signature key; the signature key itself
 * is in the leaf, so the check needs no other input from the group.
 */
import type { CredentialCheck } from "../mls/engine.js";
import { verifyDevice, type PublishedIdentity } from "./identity.js";

const label = "nexus-chat device credential v1";

/** A device's MLS credential: its address and its user's signature over its key. */
export interface DeviceCredential {
  /** `@user@instance/deviceId`. */
  identity: string;
  /** The self-signing key's certificate signature (`certifyDevice`). */
  certificate: Uint8Array;
}

/**
 * Finds the identity to trust for a user (`@user@instance`): the one pinned
 * on first contact or verified in person, or undefined if there is none
 * yet and none can be fetched. A changed master key must not be returned
 * silently; the caller warns the user first (e2ee-design.md §2).
 */
export type TrustedIdentityLookup = (user: string) => Promise<PublishedIdentity | undefined>;

function field(bytes: Uint8Array, offset: number): [Uint8Array, number] | undefined {
  if (offset + 4 > bytes.length) {
    return undefined;
  }
  const length = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength).getUint32(offset);
  const end = offset + 4 + length;
  return end > bytes.length ? undefined : [bytes.slice(offset + 4, end), end];
}

/** The credential's bytes: the label, the address and the certificate, each length-prefixed. */
export function encodeDeviceCredential(credential: DeviceCredential): Uint8Array {
  const parts = [
    new TextEncoder().encode(label),
    new TextEncoder().encode(credential.identity),
    credential.certificate,
  ];
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

/** Reads a credential made by `encodeDeviceCredential`, or undefined if it is not one. */
export function decodeDeviceCredential(bytes: Uint8Array): DeviceCredential | undefined {
  const first = field(bytes, 0);
  if (first === undefined || new TextDecoder().decode(first[0]) !== label) {
    return undefined;
  }
  const identity = field(bytes, first[1]);
  if (identity === undefined) {
    return undefined;
  }
  const certificate = field(bytes, identity[1]);
  if (certificate?.[1] !== bytes.length) {
    return undefined;
  }
  return { identity: new TextDecoder().decode(identity[0]), certificate: certificate[0] };
}

/** The user part of a device address: `@ada@example.com` from `@ada@example.com/d1`. */
export function userOf(deviceIdentity: string): string | undefined {
  const match = /^(@[^@/\s]+@[^@/\s]+)\/[^/\s]+$/.exec(deviceIdentity);
  return match?.[1];
}

/** The check for `createGroup` and `joinFromWelcome`: the device must chain to its user's trusted identity. */
export function deviceCredentialCheck(lookup: TrustedIdentityLookup): CredentialCheck {
  return async (bytes, signaturePublicKey) => {
    const credential = decodeDeviceCredential(bytes);
    const user = credential === undefined ? undefined : userOf(credential.identity);
    const trusted = user === undefined ? undefined : await lookup(user);
    if (credential === undefined || trusted === undefined) {
      return false;
    }
    try {
      await verifyDevice(trusted, {
        identity: credential.identity,
        signaturePublicKey,
        signature: credential.certificate,
      });
      return true;
    } catch {
      return false;
    }
  };
}
