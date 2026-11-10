/**
 * Key backup (docs/e2ee-design.md §5, spec §5.4). A random 256-bit backup
 * key encrypts the user's keys into blobs stored on their home instance;
 * the backup key itself is stored wrapped under each unlock method the
 * user has turned on:
 *
 * - a 24-word recovery phrase (BIP-39 encoding of 256 random bits),
 * - their password, through Argon2id with its own salt and context,
 * - a passkey's WebAuthn PRF output.
 *
 * Blobs and wrapped keys are XChaCha20-Poly1305 (libsodium) with a random
 * nonce, and the additional data names what the bytes are, so a blob
 * cannot be passed off as a wrapped key or one method's key as another's.
 * Key derivation is HKDF-SHA256 and Argon2id, as documented by their RFCs.
 */
import { hkdf } from "@noble/hashes/hkdf.js";
import { sha256 } from "@noble/hashes/sha2.js";
import { generateMnemonic, mnemonicToEntropy, validateMnemonic } from "@scure/bip39";
import { wordlist } from "@scure/bip39/wordlists/english.js";
import sodium from "libsodium-wrappers-sumo";

/** How a wrapped copy of the backup key is unlocked. */
export type UnlockMethod = "recovery-phrase" | "password" | "passkey";

/** Argon2id settings, stored beside a password-wrapped key. */
export interface PasswordParameters {
  salt: Uint8Array;
  opsLimit: number;
  memLimitBytes: number;
}

/** The design's settings; phones that cannot spare 256 MiB use 64 MiB. */
export const defaultPasswordCost = { opsLimit: 3, memLimitBytes: 256 * 1024 * 1024 } as const;
/** The lowest memory cost accepted: anything less is too cheap to guess against. */
export const minimumPasswordMemory = 64 * 1024 * 1024;

/** A backup that can't be opened: wrong key, phrase or password, or changed bytes. */
export class BackupError extends Error {
  override name = "BackupError";
}

const encoder = new TextEncoder();
const blobContext = "nexus-chat backup v1 blob";
const wrapContext = "nexus-chat backup v1 wrapped key";
const passwordContext = "nexus-chat backup v1";

/** A new random backup key. */
export async function createBackupKey(): Promise<Uint8Array> {
  await sodium.ready;
  return sodium.crypto_aead_xchacha20poly1305_ietf_keygen();
}

/** nonce ‖ XChaCha20-Poly1305(key, plaintext, additional data). */
async function seal(key: Uint8Array, plaintext: Uint8Array, additionalData: string): Promise<Uint8Array> {
  await sodium.ready;
  const nonce = sodium.randombytes_buf(sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES);
  const sealed = sodium.crypto_aead_xchacha20poly1305_ietf_encrypt(
    plaintext,
    additionalData,
    null,
    nonce,
    key,
  );
  const out = new Uint8Array(nonce.length + sealed.length);
  out.set(nonce);
  out.set(sealed, nonce.length);
  return out;
}

async function open(key: Uint8Array, bytes: Uint8Array, additionalData: string): Promise<Uint8Array> {
  await sodium.ready;
  const nonceBytes = sodium.crypto_aead_xchacha20poly1305_ietf_NPUBBYTES;
  try {
    return sodium.crypto_aead_xchacha20poly1305_ietf_decrypt(
      null,
      bytes.subarray(nonceBytes),
      additionalData,
      bytes.subarray(0, nonceBytes),
      key,
    );
  } catch {
    throw new BackupError("the backup could not be opened");
  }
}

/** Encrypts one backup blob; `name` (for example "identity") must match when opening. */
export function sealBlob(backupKey: Uint8Array, name: string, plaintext: Uint8Array): Promise<Uint8Array> {
  return seal(backupKey, plaintext, `${blobContext}\0${name}`);
}

/** Decrypts a blob made by `sealBlob` under the same name. */
export function openBlob(backupKey: Uint8Array, name: string, blob: Uint8Array): Promise<Uint8Array> {
  return open(backupKey, blob, `${blobContext}\0${name}`);
}

/** Encrypts the backup key under a wrapping key from one unlock method. */
export function wrapBackupKey(
  backupKey: Uint8Array,
  method: UnlockMethod,
  wrappingKey: Uint8Array,
): Promise<Uint8Array> {
  return seal(wrappingKey, backupKey, `${wrapContext}\0${method}`);
}

/** Recovers the backup key from a copy wrapped with `wrapBackupKey`. */
export function unwrapBackupKey(
  wrapped: Uint8Array,
  method: UnlockMethod,
  wrappingKey: Uint8Array,
): Promise<Uint8Array> {
  return open(wrappingKey, wrapped, `${wrapContext}\0${method}`);
}

const wrappingKeyFrom = (secret: Uint8Array, salt: Uint8Array, method: UnlockMethod): Uint8Array =>
  hkdf(sha256, secret, salt, encoder.encode(`${wrapContext}\0${method}`), 32);

/** A new recovery phrase: 24 words, 256 bits of entropy. */
export function createRecoveryPhrase(): string {
  return generateMnemonic(wordlist, 256);
}

/**
 * The phrase as the user typed it, tidied (case, spaces), or undefined if
 * it is not a valid phrase; the checksum catches most typing mistakes.
 */
export function normalizeRecoveryPhrase(input: string): string | undefined {
  const phrase = input.trim().toLowerCase().split(/\s+/).join(" ");
  return phrase.split(" ").length === 24 && validateMnemonic(phrase, wordlist) ? phrase : undefined;
}

/** The wrapping key for a recovery phrase. `salt` is the user's backup salt. */
export function recoveryPhraseKey(phrase: string, salt: Uint8Array): Uint8Array {
  const normalized = normalizeRecoveryPhrase(phrase);
  if (normalized === undefined) {
    throw new BackupError("that is not a recovery phrase");
  }
  return wrappingKeyFrom(mnemonicToEntropy(normalized, wordlist), salt, "recovery-phrase");
}

/** Fresh Argon2id settings with a new salt. */
export async function newPasswordParameters(
  memLimitBytes: number = defaultPasswordCost.memLimitBytes,
): Promise<PasswordParameters> {
  await sodium.ready;
  return {
    salt: sodium.randombytes_buf(sodium.crypto_pwhash_SALTBYTES),
    opsLimit: defaultPasswordCost.opsLimit,
    memLimitBytes,
  };
}

/**
 * The wrapping key for a password: Argon2id with the stored settings, then
 * HKDF with the backup context, so no login hash ever equals it.
 */
export async function passwordKey(password: string, parameters: PasswordParameters): Promise<Uint8Array> {
  await sodium.ready;
  if (
    parameters.memLimitBytes < minimumPasswordMemory ||
    parameters.opsLimit < defaultPasswordCost.opsLimit
  ) {
    throw new BackupError("these password settings are too weak");
  }
  const stretched = sodium.crypto_pwhash(
    32,
    password,
    parameters.salt,
    parameters.opsLimit,
    parameters.memLimitBytes,
    sodium.crypto_pwhash_ALG_ARGON2ID13,
  );
  return wrappingKeyFrom(stretched, encoder.encode(passwordContext), "password");
}

/** The wrapping key for a passkey, from its PRF output. `salt` is the user's backup salt. */
export function passkeyKey(prfOutput: Uint8Array, salt: Uint8Array): Uint8Array {
  if (prfOutput.length < 32) {
    throw new BackupError("the passkey gave too short a secret");
  }
  return wrappingKeyFrom(prfOutput, salt, "passkey");
}
