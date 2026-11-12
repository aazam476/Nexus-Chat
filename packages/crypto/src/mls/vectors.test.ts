/**
 * The MLS interoperability test vectors (spec §5.1), through the same
 * runner the Android app uses on Hermes. Fetch them first with
 * scripts/fetch-mls-vectors.sh.
 */
import { existsSync, readFileSync } from "node:fs";

import { runPassiveClientCase, VectorError, type PassiveClientCase } from "./vectors.js";

const directory = new URL("../../vectors/", import.meta.url);
const files = [
  "passive-client-welcome.json",
  "passive-client-handling-commit.json",
  "passive-client-random.json",
];

function load(file: string): PassiveClientCase[] {
  const path = new URL(file, directory);
  if (!existsSync(path)) {
    throw new Error(`missing ${file}: run scripts/fetch-mls-vectors.sh`);
  }
  return JSON.parse(readFileSync(path, "utf8")) as PassiveClientCase[];
}

describe.each(files)("MLS interop vectors: %s", (file) => {
  const cases = load(file);

  it("has test cases", () => {
    expect(cases.length).toBeGreaterThan(0);
  });

  it.each(cases.map((vector, index) => [index, vector.cipher_suite, vector] as const))(
    "case %i (ciphersuite %i)",
    async (_index, _suite, vector) => {
      await runPassiveClientCase(vector);
    },
  );
});

describe("a case that does not pass", () => {
  // A small case with at least one epoch and one proposal.
  const base = load("passive-client-handling-commit.json").find(
    (vector) => vector.cipher_suite === 1 && vector.epochs.some((epoch) => epoch.proposals.length > 0),
  );
  if (base === undefined) {
    throw new Error("the vectors have no suitable case");
  }
  const failing = async (changed: Partial<PassiveClientCase>): Promise<unknown> =>
    runPassiveClientCase({ ...base, ...changed }).catch((error: unknown) => error);

  it("is refused for undecodable or misplaced messages", async () => {
    expect(await failing({ welcome: "00" })).toBeInstanceOf(VectorError);
    expect(await failing({ welcome: base.key_package })).toBeInstanceOf(VectorError);
    const epochs = [{ ...base.epochs[0], proposals: [base.welcome], commit: "" }];
    expect(await failing({ epochs: epochs as PassiveClientCase["epochs"] })).toBeInstanceOf(VectorError);
  });

  it("is refused when an epoch authenticator differs", async () => {
    expect(await failing({ initial_epoch_authenticator: "00" })).toMatchObject({
      message: "the epoch authenticator differs after joining",
    });
    const [first] = base.epochs;
    const wrong = [
      { proposals: first?.proposals ?? [], commit: first?.commit ?? "", epoch_authenticator: "00" },
    ];
    expect(await failing({ epochs: wrong })).toMatchObject({
      message: "the epoch authenticator differs in epoch 1",
    });
  });
});
