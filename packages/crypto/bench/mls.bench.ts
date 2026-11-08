/**
 * Measures the MLS operations that grow with group size, for the M3
 * review of the MLS library (ADR 0002): how long a device takes to join,
 * to apply a commit, and to encrypt and decrypt, in groups of 10 to 2,000
 * devices. Run with `pnpm --filter @nexus-chat/crypto bench`; results go
 * to stdout as a table.
 */
import { ed25519 } from "@noble/curves/ed25519.js";
import { performance } from "node:perf_hooks";

import {
  addMembers,
  createGroup,
  createMember,
  encrypt,
  joinFromWelcome,
  receive,
  removeMembers,
  updateKeys,
} from "../src/mls/engine.js";

const sizes = (process.env.MLS_BENCH_SIZES ?? "10,100,1000,2000").split(",").map(Number);
const text = (value: string): Uint8Array => new TextEncoder().encode(value);

async function timed<T>(work: () => Promise<T>): Promise<[T, number]> {
  const start = performance.now();
  const result = await work();
  return [result, performance.now() - start];
}

interface Row {
  devices: number;
  keyPackagesMs: number;
  addAllMs: number;
  welcomeKiB: number;
  joinMs: number;
  commitMs: number;
  applyCommitMs: number;
  encryptMs: number;
  decryptMs: number;
}

/** A group of `size` devices, seen by its creator and by the last device to join. */
async function measure(size: number): Promise<Row> {
  const [devices, keyPackagesMs] = await timed(() =>
    Promise.all(Array.from({ length: size }, (_, index) => createMember(text(`device-${String(index)}`)))),
  );
  const [creator, ...others] = devices;
  const lastDevice = others.at(-1);
  if (creator === undefined || lastDevice === undefined) {
    throw new Error("a group needs at least two devices");
  }
  const start = await createGroup(text(`bench-${String(size)}`), creator);
  const [added, addAllMs] = await timed(() =>
    addMembers(
      start,
      others.map((device) => device.keyPackage),
    ),
  );
  const welcome = added.welcome;
  if (welcome === undefined) {
    throw new Error("adding devices produced no Welcome");
  }
  const [joined, joinMs] = await timed(() => joinFromWelcome(welcome, lastDevice));
  // A commit with an update path, as every removal or key refresh makes.
  const [committed, commitMs] = await timed(() => removeMembers(added.group, [1]));
  const [applied, applyCommitMs] = await timed(() => receive(joined, committed.commit));
  const [sent, encryptMs] = await timed(() => encrypt(committed.group, text("a typical chat message")));
  const [, decryptMs] = await timed(() => receive(applied.group, sent.message));
  return {
    devices: size,
    keyPackagesMs,
    addAllMs,
    welcomeKiB: Math.round(welcome.length / 1024),
    joinMs,
    commitMs,
    applyCommitMs,
    encryptMs,
    decryptMs,
  };
}

/** Pure-JavaScript Ed25519 verification: the cost where native WebCrypto is missing. */
function pureJsVerifyMs(): number {
  const secret = ed25519.utils.randomSecretKey();
  const publicKey = ed25519.getPublicKey(secret);
  const message = new Uint8Array(200);
  const signature = ed25519.sign(message, secret);
  const runs = 200;
  const start = performance.now();
  for (let index = 0; index < runs; index++) {
    ed25519.verify(signature, message, publicKey);
  }
  return (performance.now() - start) / runs;
}

it("measures MLS at growing group sizes", async () => {
  const rows: Row[] = [];
  for (const size of sizes) {
    rows.push(await measure(size));
  }
  console.table(
    rows.map((row) =>
      Object.fromEntries(Object.entries(row).map(([key, value]) => [key, Math.round(Number(value))])),
    ),
  );
  console.log(
    `Pure-JS Ed25519 verify: ${pureJsVerifyMs().toFixed(3)} ms each; WebCrypto is used whenever available.`,
  );
  expect(rows).toHaveLength(sizes.length);
  // Keep updateKeys referenced: it is the other path-carrying commit.
  expect(typeof updateKeys).toBe("function");
});
