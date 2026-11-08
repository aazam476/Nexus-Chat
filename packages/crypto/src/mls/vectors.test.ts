/**
 * The MLS interoperability test vectors (spec §5.1) from
 * github.com/mlswg/mls-implementations, run against the ts-mls build we
 * ship. The passive-client vectors are end to end: join from a Welcome
 * made by another implementation, then apply its proposals and commits,
 * and agree on every epoch's authenticator (RFC 9420 §8.7).
 *
 * Fetch them first with scripts/fetch-mls-vectors.sh.
 */
import { existsSync, readFileSync } from "node:fs";
import {
  acceptAll,
  decodeMlsMessage,
  getCiphersuiteImpl,
  joinGroup,
  makePskIndex,
  processMessage,
  type ClientState,
  type CiphersuiteImpl,
  type MLSMessage,
} from "ts-mls";
import { getCiphersuiteFromId, type CiphersuiteId } from "ts-mls/crypto/ciphersuite.js";
import { decodeRatchetTree } from "ts-mls/ratchetTree.js";

/** One passive-client test case, as the vector files spell it. */
interface PassiveClientCase {
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

const directory = new URL("../../vectors/", import.meta.url);
const files = [
  "passive-client-welcome.json",
  "passive-client-handling-commit.json",
  "passive-client-random.json",
];

function hex(value: string): Uint8Array {
  return Uint8Array.from(Buffer.from(value, "hex"));
}

function message(value: string): MLSMessage {
  const decoded = decodeMlsMessage(hex(value), 0);
  if (decoded === undefined) {
    throw new Error("vector holds an undecodable message");
  }
  return decoded[0];
}

function load(file: string): PassiveClientCase[] {
  const path = new URL(file, directory);
  if (!existsSync(path)) {
    throw new Error(`missing ${file}: run scripts/fetch-mls-vectors.sh`);
  }
  return JSON.parse(readFileSync(path, "utf8")) as PassiveClientCase[];
}

async function join(
  vector: PassiveClientCase,
  cs: CiphersuiteImpl,
  psks: Record<string, Uint8Array>,
): Promise<ClientState> {
  const keyPackage = message(vector.key_package);
  const welcome = message(vector.welcome);
  if (keyPackage.wireformat !== "mls_key_package" || welcome.wireformat !== "mls_welcome") {
    throw new Error("vector holds the wrong message types");
  }
  const tree = vector.ratchet_tree === null ? undefined : decodeRatchetTree(hex(vector.ratchet_tree), 0)?.[0];
  const secrets = {
    initPrivateKey: hex(vector.init_priv),
    hpkePrivateKey: hex(vector.encryption_priv),
    signaturePrivateKey: hex(vector.signature_priv),
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
    throw new Error("vector holds a non-group message");
  }
  const result = await processMessage(incoming, state, makePskIndex(state, psks), acceptAll, cs);
  return result.newState;
}

async function runCase(vector: PassiveClientCase): Promise<void> {
  const cs = await getCiphersuiteImpl(getCiphersuiteFromId(vector.cipher_suite as CiphersuiteId));
  const psks = Object.fromEntries(
    vector.external_psks.map((psk) => [Buffer.from(hex(psk.psk_id)).toString("base64"), hex(psk.psk)]),
  );
  let state = await join(vector, cs, psks);
  expect(Buffer.from(state.keySchedule.epochAuthenticator).toString("hex")).toBe(
    vector.initial_epoch_authenticator,
  );
  for (const epoch of vector.epochs) {
    for (const proposal of epoch.proposals) {
      state = await apply(state, proposal, cs, psks);
    }
    state = await apply(state, epoch.commit, cs, psks);
    expect(Buffer.from(state.keySchedule.epochAuthenticator).toString("hex")).toBe(epoch.epoch_authenticator);
  }
}

describe.each(files)("MLS interop vectors: %s", (file) => {
  const cases = load(file);

  it("has test cases", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases.map((vector, index) => [index, vector.cipher_suite, vector] as const))(
    "case %i (ciphersuite %i)",
    async (_index, _suite, vector) => {
      await runCase(vector);
    },
  );
});
