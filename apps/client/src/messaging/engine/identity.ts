/**
 * The user's identity on this device (docs/e2ee-design.md §2 and §5).
 *
 * On a new account's first device, `setUpIdentity` makes the master and
 * self-signing keys, publishes their public halves, and backs the private
 * halves up under a new recovery phrase, which the app shows once and the
 * user must confirm. A later device of the same user finds the published
 * identity and has to restore it, from the phrase or another device.
 */
import {
  createBackupKey,
  createIdentity,
  createRecoveryPhrase,
  openBlob,
  recoveryPhraseKey,
  sealBlob,
  unwrapBackupKey,
  wrapBackupKey,
  type SigningKeyPair,
} from "@nexus-chat/crypto";

import type { LocalDatabase } from "../../storage/localDatabase";
import type { MessagingApi } from "./api";
import { fromBase64, toBase64, utf8 } from "./bytes";

/** What this device keeps of the user's identity. */
export interface StoredIdentity {
  masterPublicKey: Uint8Array;
  selfSigningKey: SigningKeyPair;
  selfSigningSignature: Uint8Array;
  backupKey: Uint8Array;
}

/** Where identity setup stands for this device. */
export type IdentityResult =
  { status: "ready"; identity: StoredIdentity; recoveryPhrase?: string } | { status: "needs-restore" };

/** Why a restore failed. */
export class RestoreError extends Error {
  override name = "RestoreError";
}

const collection = "keys";
const record = "identity";

/** Named byte fields as JSON with base64 values. */
function encodeFields(fields: Record<string, Uint8Array>): Uint8Array {
  return utf8(
    JSON.stringify(
      Object.fromEntries(Object.entries(fields).map(([name, value]) => [name, toBase64(value)])),
    ),
  );
}

/** Reads `encodeFields` output; a missing field reads as empty bytes. */
function decodeFields(bytes: Uint8Array): (name: string) => Uint8Array {
  const fields = JSON.parse(new TextDecoder().decode(bytes)) as Record<string, string>;
  return (name) => fromBase64(fields[name] ?? "");
}

function encode(identity: StoredIdentity): Uint8Array {
  return encodeFields({
    masterPublicKey: identity.masterPublicKey,
    selfSigningPublicKey: identity.selfSigningKey.publicKey,
    selfSigningSecretKey: identity.selfSigningKey.secretKey,
    selfSigningSignature: identity.selfSigningSignature,
    backupKey: identity.backupKey,
  });
}

function decode(bytes: Uint8Array): StoredIdentity {
  const read = decodeFields(bytes);
  return {
    masterPublicKey: read("masterPublicKey"),
    selfSigningKey: { publicKey: read("selfSigningPublicKey"), secretKey: read("selfSigningSecretKey") },
    selfSigningSignature: read("selfSigningSignature"),
    backupKey: read("backupKey"),
  };
}

/**
 * The identity blob in the key backup. It also holds the master key's
 * secret, needed only to sign a new self-signing key later; devices do not
 * keep it (§2).
 */
function encodeBackup(identity: StoredIdentity, masterSecretKey: Uint8Array): Uint8Array {
  return encodeFields({
    masterSecretKey,
    selfSigningPublicKey: identity.selfSigningKey.publicKey,
    selfSigningSecretKey: identity.selfSigningKey.secretKey,
    selfSigningSignature: identity.selfSigningSignature,
  });
}

/** The identity this device holds, if any. */
export async function loadIdentity(database: LocalDatabase): Promise<StoredIdentity | undefined> {
  const stored = await database.get(collection, record);
  return stored === undefined ? undefined : decode(stored);
}

/**
 * Makes sure this device holds the user's identity: already here, newly
 * created (with a recovery phrase to show), or still to be restored.
 */
export async function setUpIdentity(
  api: MessagingApi,
  database: LocalDatabase,
  userId: string,
): Promise<IdentityResult> {
  const local = await loadIdentity(database);
  if (local !== undefined) {
    return { status: "ready", identity: local };
  }
  if ((await api.getIdentity(userId)) !== undefined) {
    return { status: "needs-restore" };
  }
  const created = await createIdentity();
  const identity: StoredIdentity = {
    masterPublicKey: created.masterKey.publicKey,
    selfSigningKey: created.selfSigningKey,
    selfSigningSignature: created.selfSigningSignature,
    backupKey: await createBackupKey(),
  };
  const recoveryPhrase = createRecoveryPhrase();
  const { salt } = await api.getBackup();
  const sealed = await sealBlob(
    identity.backupKey,
    record,
    encodeBackup(identity, created.masterKey.secretKey),
  );
  await api.putBlob(record, sealed);
  await api.putWrappedKey(
    "recovery-phrase",
    await wrapBackupKey(identity.backupKey, "recovery-phrase", recoveryPhraseKey(recoveryPhrase, salt)),
  );
  await api.publishIdentity({
    masterPublicKey: identity.masterPublicKey,
    selfSigningPublicKey: identity.selfSigningKey.publicKey,
    selfSigningSignature: identity.selfSigningSignature,
  });
  await database.put(collection, record, encode(identity));
  return { status: "ready", identity, recoveryPhrase };
}

/** Restores the identity on this device from the recovery phrase. */
export async function restoreWithPhrase(
  api: MessagingApi,
  database: LocalDatabase,
  userId: string,
  phrase: string,
): Promise<StoredIdentity> {
  const backup = await api.getBackup();
  const wrapped = backup.wrapped.get("recovery-phrase");
  const sealed = await api.getBlob(record);
  const published = await api.getIdentity(userId);
  if (wrapped === undefined || sealed === undefined || published === undefined) {
    throw new RestoreError("there is no backup to restore");
  }
  const backupKey = await unwrapBackupKey(wrapped, "recovery-phrase", recoveryPhraseKey(phrase, backup.salt));
  const read = decodeFields(await openBlob(backupKey, record, sealed));
  const identity: StoredIdentity = {
    masterPublicKey: published.masterPublicKey,
    selfSigningKey: { publicKey: read("selfSigningPublicKey"), secretKey: read("selfSigningSecretKey") },
    selfSigningSignature: read("selfSigningSignature"),
    backupKey,
  };
  if (toBase64(identity.selfSigningKey.publicKey) !== toBase64(published.selfSigningPublicKey)) {
    throw new RestoreError("the backup does not match the published identity");
  }
  await database.put(collection, record, encode(identity));
  return identity;
}
