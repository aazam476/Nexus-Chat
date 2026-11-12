/**
 * The MLS interoperability vectors, run inside the app (spec §5.1): CI's
 * Android build opens the self-test screen, which fetches the vector files
 * from a server on the CI machine and runs every case on Hermes, with the
 * device's native crypto. Store builds have no such screen.
 */
import { runPassiveClientCase, type PassiveClientCase } from "@nexus-chat/crypto";

/** The vector files, as scripts/fetch-mls-vectors.sh downloads them. */
export const vectorFiles = [
  "passive-client-welcome.json",
  "passive-client-handling-commit.json",
  "passive-client-random.json",
];

/** How far the run is, and which cases failed. */
export interface VectorProgress {
  done: number;
  total: number;
  failures: string[];
}

/** What a run needs; the defaults are the app's own. */
export interface VectorRunner {
  fetch: (url: string) => Promise<{ ok: boolean; json: () => Promise<unknown> }>;
  run: (vector: PassiveClientCase) => Promise<void>;
}

const defaults: VectorRunner = { fetch: (url) => fetch(url), run: runPassiveClientCase };

async function loadAll(baseUrl: string, runner: VectorRunner): Promise<[string, PassiveClientCase][]> {
  const cases: [string, PassiveClientCase][] = [];
  for (const file of vectorFiles) {
    const response = await runner.fetch(`${baseUrl}/${file}`);
    if (!response.ok) {
      throw new Error(`could not fetch ${file}`);
    }
    const loaded = (await response.json()) as PassiveClientCase[];
    loaded.forEach((vector, index) => cases.push([`${file} #${String(index)}`, vector]));
  }
  return cases;
}

/** Runs every case, reporting progress after each; resolves with the result. */
export async function runVectors(
  baseUrl: string,
  report: (progress: VectorProgress) => void,
  runner: VectorRunner = defaults,
): Promise<VectorProgress> {
  const cases = await loadAll(baseUrl, runner);
  const progress: VectorProgress = { done: 0, total: cases.length, failures: [] };
  for (const [name, vector] of cases) {
    try {
      await runner.run(vector);
    } catch (cause) {
      progress.failures.push(`${name}: ${cause instanceof Error ? cause.message : String(cause)}`);
    }
    progress.done += 1;
    report({ ...progress, failures: [...progress.failures] });
  }
  return progress;
}
