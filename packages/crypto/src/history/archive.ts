/**
 * The archive copy of a message (docs/e2ee-design.md §4): the plaintext
 * sealed with the channel's history key for the epoch, so the sender's
 * other devices, search, and (where allowed) new members can read it after
 * the live MLS copy is gone.
 *
 *   historyKey = MLS-Exporter("nexus-chat history v1", channelId ‖ epoch, 32)
 *
 * Inside the seal travel the sender's credential and MLS signature key and
 * their signature over channel, epoch, message ID, plaintext and
 * credential, so an archive copy is as authenticated as the live one even
 * for a reader who never saw that epoch's ratchet tree. AES-256-GCM
 * (WebCrypto) with a random nonce; the channel, epoch and message ID are
 * also bound as additional data.
 */
import { lengthPrefixed, readFields, uint64, utf8 } from "../encoding.js";
import type { CredentialCheck, Group } from "../mls/engine.js";
import { epoch } from "../mls/engine.js";
import { exportSecret, ownLeaf, signAsMember, verifyMemberSignature } from "../mls/memberKeys.js";

const historyLabel = "nexus-chat history v1";
const signatureLabel = "nexus-chat archive v1";
const version = 1;

/** Where an archive copy belongs. */
export interface ArchiveAddress {
  channelId: Uint8Array;
  messageId: Uint8Array;
}

/** An archive copy as stored: its epoch (to pick the key) and the sealed bytes. */
export interface ArchiveCopy {
  epoch: bigint;
  sealed: Uint8Array;
}

/** An opened archive copy: the plaintext and who sent it. */
export interface OpenedArchive {
  plaintext: Uint8Array;
  /** The sender's credential identity bytes, already checked. */
  sender: Uint8Array;
}

/** Why an archive copy was refused. */
export class ArchiveError extends Error {
  override name = "ArchiveError";
}

/** The channel's history key for the group's current epoch. */
export function historyKey(group: Group, channelId: Uint8Array): Promise<Uint8Array> {
  return exportSecret(group, historyLabel, lengthPrefixed(channelId, uint64(epoch(group))), 32);
}

const additionalData = (address: ArchiveAddress, at: bigint): Uint8Array =>
  lengthPrefixed(utf8(signatureLabel), address.channelId, uint64(at), address.messageId);

const importKey = (key: Uint8Array, usage: KeyUsage): Promise<CryptoKey> =>
  crypto.subtle.importKey("raw", new Uint8Array(key), "AES-GCM", false, [usage]);

/** Seals `plaintext` as the archive copy of a message this device sends now. */
export async function sealArchive(
  group: Group,
  address: ArchiveAddress,
  plaintext: Uint8Array,
): Promise<ArchiveCopy> {
  const at = epoch(group);
  const sender = ownLeaf(group);
  const signed = lengthPrefixed(additionalData(address, at), plaintext, sender.identity);
  const signature = await signAsMember(group, signatureLabel, signed);
  const inner = lengthPrefixed(plaintext, sender.identity, sender.signaturePublicKey, signature);
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce, additionalData: new Uint8Array(additionalData(address, at)) },
    await importKey(await historyKey(group, address.channelId), "encrypt"),
    new Uint8Array(inner),
  );
  const sealed = new Uint8Array(1 + nonce.length + ciphertext.byteLength);
  sealed[0] = version;
  sealed.set(nonce, 1);
  sealed.set(new Uint8Array(ciphertext), 1 + nonce.length);
  return { epoch: at, sealed };
}

async function decrypt(copy: ArchiveCopy, address: ArchiveAddress, key: Uint8Array): Promise<Uint8Array> {
  if (copy.sealed[0] !== version || copy.sealed.length < 1 + 12 + 16) {
    throw new ArchiveError("this is not an archive copy this version can read");
  }
  try {
    return new Uint8Array(
      await crypto.subtle.decrypt(
        {
          name: "AES-GCM",
          iv: copy.sealed.slice(1, 13),
          additionalData: new Uint8Array(additionalData(address, copy.epoch)),
        },
        await importKey(key, "decrypt"),
        copy.sealed.slice(13),
      ),
    );
  } catch {
    throw new ArchiveError("the archive copy could not be decrypted");
  }
}

/**
 * Opens an archive copy with the history key of its epoch. `check` vets
 * the sender's credential and key, as for any group member.
 */
export async function openArchive(
  copy: ArchiveCopy,
  address: ArchiveAddress,
  key: Uint8Array,
  check: CredentialCheck,
): Promise<OpenedArchive> {
  const fields = readFields(await decrypt(copy, address, key), 4);
  if (fields === undefined) {
    throw new ArchiveError("the archive copy is malformed");
  }
  const [plaintext, sender, signaturePublicKey, signature] = fields as [
    Uint8Array,
    Uint8Array,
    Uint8Array,
    Uint8Array,
  ];
  const signed = lengthPrefixed(additionalData(address, copy.epoch), plaintext, sender);
  if (!(await verifyMemberSignature(signaturePublicKey, signatureLabel, signed, signature))) {
    throw new ArchiveError("the sender's signature does not match");
  }
  if (!(await check(sender, signaturePublicKey))) {
    throw new ArchiveError("the sender's credential could not be verified");
  }
  return { plaintext, sender };
}
