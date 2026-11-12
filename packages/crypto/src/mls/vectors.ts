/**
 * Runs the MLS interoperability test vectors (spec §5.1) from
 * github.com/mlswg/mls-implementations against the ts-mls build we ship.
 * The passive-client vectors are end to end: join from a Welcome made by
 * another implementation, then apply its proposals and commits, and agree
 * on every epoch's authenticator (RFC 9420 §8.7).
 *
 * It lives here rather than in a test so the same check runs wherever the
 * app runs: in Node for the package's tests, and on Hermes inside the
 * Android app's end-to-end build.
 */
import { bytesToHex, hexToBytes } from "@noble/hashes/utils.js";
import {
  acceptAll,
  decodeMlsMessage,
  getCiphersuiteImpl,
  joinGroup,
  makePskIndex,
  processMessage,
  type CiphersuiteImpl,
  type ClientState,
  type MLSMessage,
} from "ts-mls";
import { getCiphersuiteFromId, type CiphersuiteId } from "ts-mls/crypto/ciphersuite.js";
import { decodeRatchetTree } from "ts-mls/ratchetTree.js";

/** One passive-client test case, as the vector files spell it. */
export interface PassiveClientCase {
  cipher_suite: number;
  external_psks: { psk_id: string; psk: string }[];
  key_package: string;
  signature_priv: string;
  encryption_priv: string;
  init_priv: string;
  welcome: string;
  ratchet_tree: string | null;
  initial_epoch_authenticator: string;
  epochs: { proposals: string[]; commit: string; epoch_authenticator: string }[];
}

/** Why a test case did not pass. */
export class VectorError extends Error {
  override name = "VectorError";
}

function message(value: string): MLSMessage {
  const decoded = decodeMlsMessage(hexToBytes(value), 0);
  if (decoded === undefined) {
    throw new VectorError("the vector holds an undecodable message");
  }
  return decoded[0];
}

/** The pre-shared keys by base64 ID, as ts-mls looks them up. */
function pskTable(vector: PassiveClientCase): Record<string, Uint8Array> {
  const base64 = (bytes: Uint8Array): string => btoa(String.fromCharCode(...bytes));
  return Object.fromEntries(
    vector.external_psks.map((psk) => [base64(hexToBytes(psk.psk_id)), hexToBytes(psk.psk)]),
  );
}

async function join(
  vector: PassiveClientCase,
  cs: CiphersuiteImpl,
  psks: Record<string, Uint8Array>,
): Promise<ClientState> {
  const keyPackage = message(vector.key_package);
  const welcome = message(vector.welcome);
  if (keyPackage.wireformat !== "mls_key_package" || welcome.wireformat !== "mls_welcome") {
    throw new VectorError("the vector holds the wrong message types");
  }
  const tree =
    vector.ratchet_tree === null ? undefined : decodeRatchetTree(hexToBytes(vector.ratchet_tree), 0)?.[0];
  const secrets = {
    initPrivateKey: hexToBytes(vector.init_priv),
    hpkePrivateKey: hexToBytes(vector.encryption_priv),
    signaturePrivateKey: hexToBytes(vector.signature_priv),
  };
  return joinGroup(welcome.welcome, keyPackage.keyPackage, secrets, makePskIndex(undefined, psks), cs, tree);
}

async function apply(
  state: ClientState,
  bytes: string,
  cs: CiphersuiteImpl,
  psks: Record<string, Uint8Array>,
): Promise<ClientState> {
  const incoming = message(bytes);
  if (incoming.wireformat !== "mls_public_message" && incoming.wireformat !== "mls_private_message") {
    throw new VectorError("the vector holds a non-group message");
  }
  return (await processMessage(incoming, state, makePskIndex(state, psks), acceptAll, cs)).newState;
}

function expectAuthenticator(state: ClientState, expected: string, where: string): void {
  if (bytesToHex(state.keySchedule.epochAuthenticator) !== expected) {
    throw new VectorError(`the epoch authenticator differs ${where}`);
  }
}

/** Runs one passive-client case; throws a VectorError when it does not pass. */
export async function runPassiveClientCase(vector: PassiveClientCase): Promise<void> {
  const cs = await getCiphersuiteImpl(getCiphersuiteFromId(vector.cipher_suite as CiphersuiteId));
  const psks = pskTable(vector);
  let state = await join(vector, cs, psks);
  expectAuthenticator(state, vector.initial_epoch_authenticator, "after joining");
  for (const [index, epoch] of vector.epochs.entries()) {
    for (const proposal of epoch.proposals) {
      state = await apply(state, proposal, cs, psks);
    }
    state = await apply(state, epoch.commit, cs, psks);
    expectAuthenticator(state, epoch.epoch_authenticator, `in epoch ${String(index + 1)}`);
  }
}
