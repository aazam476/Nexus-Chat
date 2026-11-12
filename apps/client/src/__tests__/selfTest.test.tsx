import type { PassiveClientCase } from "@nexus-chat/crypto";

import { runVectors, vectorFiles, type VectorRunner } from "../selfTest/vectorRun";

const vector = (suite: number): PassiveClientCase => ({ cipher_suite: suite }) as PassiveClientCase;

function runner(failing: number[]): VectorRunner {
  return {
    fetch: (url) =>
      Promise.resolve({
        ok: !url.endsWith("missing.json"),
        json: () =>
          Promise.resolve(url.endsWith(vectorFiles[0] ?? "") ? [vector(1), vector(2)] : [vector(3)]),
      }),
    run: (item) =>
      failing.includes(item.cipher_suite) ? Promise.reject(new Error("differs")) : Promise.resolve(),
  };
}

describe("running the MLS vectors in the app", () => {
  it("runs every case of every file and reports each failure", async () => {
    const reports: number[] = [];
    const result = await runVectors("http://vectors", (progress) => reports.push(progress.done), runner([2]));
    expect(result).toEqual({
      done: 4,
      total: 4,
      failures: ["passive-client-welcome.json #1: differs"],
    });
    expect(reports).toEqual([1, 2, 3, 4]);
    await expect(
      runVectors("http://vectors", () => undefined, {
        ...runner([]),
        run: () => Promise.reject(new Error("x")),
        fetch: () => Promise.resolve({ ok: false, json: () => Promise.resolve([]) }),
      }),
    ).rejects.toThrow("could not fetch passive-client-welcome.json");
    const strings = await runVectors("http://vectors", () => undefined, {
      ...runner([]),
      // eslint-disable-next-line @typescript-eslint/prefer-promise-reject-errors -- a library may reject with anything
      run: () => Promise.reject("no"),
    });
    expect(strings.failures).toContain("passive-client-random.json #0: no");
  });

  it("fetches from the vector server by default", async () => {
    const fetching = jest.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 404 }));
    await expect(runVectors("http://vectors", () => undefined)).rejects.toThrow("could not fetch");
    expect(fetching).toHaveBeenCalledWith("http://vectors/passive-client-welcome.json");
    fetching.mockRestore();
  });
});
