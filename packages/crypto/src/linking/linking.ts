/**
 * Linking a new device (docs/e2ee-design.md §2). The new device makes a
 * fresh X25519 key; the existing device answers with its own, and both
 * derive session keys with libsodium's `crypto_kx`. The user checks the
 * same comparison code on both screens, then the existing device sends the
 * self-signing key and the backup key over that session.
 *
 * Two ways to start:
 *
 * - **QR code:** the existing device scans the new device's key, so the
 *   key reaches it unchanged.
 * - **Through the server** (no camera): the new device first sends only a
 *   commitment (a hash) to its key and reveals the key after the answer.
 *   Someone relaying the messages has to pick their own keys before seeing
 *   the real ones, so they cannot search for keys that make both screens
 *   show the same code; each attempt matches with a chance of one in
 *   10^24 (6 groups of 4 digits).
 *
 * The comparison code always covers both keys and the new device's nonce.
 */
import { sha256, sha512 } from "@noble/hashes/sha2.js";
import sodium from "libsodium-wrappers-sumo";

/** Why linking stopped. */
export class LinkingError extends Error {
  override name = "LinkingError";
}

/** What the new device shows or sends first. */
export interface LinkOffer {
  /** For the QR code: `nxlink1:` and the key and nonce in base64url. */
  qrCode: string;
  /** For linking through the server: a commitment to the key and nonce. */
  commitment: Uint8Array;
}

/** The new device's half-finished side, kept in memory only. */
export interface NewDeviceLink {
  publicKey: Uint8Array;
  secretKey: Uint8Array;
  nonce: Uint8Array;
}

/** The existing device's half-finished side. */
export interface ExistingDeviceLink {
  publicKey: Uint8Array;
  secretKey: Uint8Array;
  /** Set when started from a QR code. */
  scanned?: { publicKey: Uint8Array; nonce: Uint8Array };
  /** Set when started through the server. */
  commitment?: Uint8Array;
}

/** The new device's key and nonce, sent after the answer. */
export interface LinkReveal {
  publicKey: Uint8Array;
  nonce: Uint8Array;
}

/** A finished exchange: the code to compare, and keys to send and receive with. */
export interface LinkSession {
  comparisonCode: string;
  sendKey: Uint8Array;
  receiveKey: Uint8Array;
}

const qrPrefix = "nxlink1:";
const commitmentLabel = "nexus-chat link v1 commitment";
const codeLabel = "nexus-chat link v1 comparison code";
const transferLabel = "nexus-chat link v1 transfer";
const nonceBytes = 16;

const joined = (...parts: Uint8Array[]): Uint8Array => {
  const out = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
};
const label = (value: string): Uint8Array => new TextEncoder().encode(value);

const commit = (publicKey: Uint8Array, nonce: Uint8Array): Uint8Array =>
  sha256(joined(label(commitmentLabel), publicKey, nonce));

/** 6 groups of 4 digits from both keys and the nonce. */
function comparisonCode(nonce: Uint8Array, newKey: Uint8Array, existingKey: Uint8Array): string {
  const digest = sha512(joined(label(codeLabel), nonce, newKey, existingKey));
  const view = new DataView(digest.buffer, digest.byteOffset, digest.byteLength);
  return Array.from({ length: 6 }, (_, group) =>
    String(view.getUint32(group * 4) % 10_000).padStart(4, "0"),
  ).join(" ");
}

/** Starts linking on the new device. */
export async function startLinking(): Promise<{ offer: LinkOffer; link: NewDeviceLink }> {
  await sodium.ready;
  const { publicKey, privateKey } = sodium.crypto_kx_keypair();
  const nonce = sodium.randombytes_buf(nonceBytes);
  const qrCode =
    qrPrefix + sodium.to_base64(joined(publicKey, nonce), sodium.base64_variants.URLSAFE_NO_PADDING);
  return {
    offer: { qrCode, commitment: commit(publicKey, nonce) },
    link: { publicKey, secretKey: privateKey, nonce },
  };
}

/** The key and nonce in a scanned QR code. */
function readQrCode(qrCode: string): { publicKey: Uint8Array; nonce: Uint8Array } {
  let bytes: Uint8Array | undefined;
  if (qrCode.startsWith(qrPrefix)) {
    try {
      bytes = sodium.from_base64(qrCode.slice(qrPrefix.length), sodium.base64_variants.URLSAFE_NO_PADDING);
    } catch {
      bytes = undefined;
    }
  }
  const keyBytes = sodium.crypto_kx_PUBLICKEYBYTES;
  if (bytes?.length !== keyBytes + nonceBytes) {
    throw new LinkingError("that is not a Nexus Chat linking code");
  }
  return { publicKey: bytes.slice(0, keyBytes), nonce: bytes.slice(keyBytes) };
}

/**
 * Answers on the existing device, from a scanned QR code or a commitment
 * received through the server. Send back `answer`, the existing device's key.
 */
export async function answerLinking(
  offer: { qrCode: string } | { commitment: Uint8Array },
): Promise<{ answer: Uint8Array; link: ExistingDeviceLink }> {
  await sodium.ready;
  const started =
    "qrCode" in offer ? { scanned: readQrCode(offer.qrCode) } : { commitment: offer.commitment };
  const { publicKey, privateKey } = sodium.crypto_kx_keypair();
  const link: ExistingDeviceLink = { publicKey, secretKey: privateKey, ...started };
  return { answer: publicKey, link };
}

/**
 * Finishes on the new device once the answer arrives. Send `reveal` to the
 * existing device (it ignores it after a QR scan) and show the code.
 */
export async function finishLinking(
  link: NewDeviceLink,
  answer: Uint8Array,
): Promise<{ reveal: LinkReveal; session: LinkSession }> {
  await sodium.ready;
  let keys: { sharedRx: Uint8Array; sharedTx: Uint8Array };
  try {
    keys = sodium.crypto_kx_client_session_keys(link.publicKey, link.secretKey, answer);
  } catch {
    throw new LinkingError("the other device's answer is not a key");
  }
  return {
    reveal: { publicKey: link.publicKey, nonce: link.nonce },
    session: {
      comparisonCode: comparisonCode(link.nonce, link.publicKey, answer),
      sendKey: keys.sharedTx,
      receiveKey: keys.sharedRx,
    },
  };
}

const sameBytes = (a: Uint8Array, b: Uint8Array): boolean =>
  a.length === b.length && a.every((byte, index) => byte === b[index]);

/** Finishes on the existing device with the new device's reveal. */
export async function confirmLinking(link: ExistingDeviceLink, reveal: LinkReveal): Promise<LinkSession> {
  await sodium.ready;
  const expected = link.scanned;
  const matches =
    expected === undefined
      ? sameBytes(commit(reveal.publicKey, reveal.nonce), link.commitment ?? new Uint8Array(0))
      : sameBytes(reveal.publicKey, expected.publicKey) && sameBytes(reveal.nonce, expected.nonce);
  if (!matches) {
    throw new LinkingError("the new device's key is not the one it offered");
  }
  const keys = sodium.crypto_kx_server_session_keys(link.publicKey, link.secretKey, reveal.publicKey);
  return {
    comparisonCode: comparisonCode(reveal.nonce, reveal.publicKey, link.publicKey),
    sendKey: keys.sharedTx,
    receiveKey: keys.sharedRx,
  };
}

/** Encrypts something for the other device; send only after the codes matched. */
export async function sealForLinkedDevice(session: LinkSession, plaintext: Uint8Array): Promise<Uint8Array> {
  await sodium.ready;
  const nonce = sodium.randombytes_buf(sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES);
  return joined(
    nonce,
    sodium.crypto_aead_xchacha20poly1305_ietf_encrypt(plaintext, transferLabel, null, nonce, session.sendKey),
  );
}

/** Decrypts what the other device sent with `sealForLinkedDevice`. */
export async function openFromLinkedDevice(session: LinkSession, sealed: Uint8Array): Promise<Uint8Array> {
  await sodium.ready;
  const split = sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES;
  try {
    return sodium.crypto_aead_xchacha20poly1305_ietf_decrypt(
      null,
      sealed.subarray(split),
      transferLabel,
      sealed.subarray(0, split),
      session.receiveKey,
    );
  } catch {
    throw new LinkingError("the transfer could not be opened");
  }
}
