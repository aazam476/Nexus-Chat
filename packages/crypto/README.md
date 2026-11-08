# @nexus-chat/crypto

End-to-end encryption for Nexus Chat (spec §5): MLS groups today; device
keys, key backup and attachment encryption follow in milestone M3.

## Files to read first

1. `src/mls/engine.ts`: every MLS operation the app uses, bytes in and
   bytes out, on top of ts-mls (ADR 0002).
2. `src/mls/vectors.test.ts`: the official interop test vectors, which
   every build must pass.
3. `bench/mls.bench.ts`: how join, commit and message costs grow with the
   group (results in ADR 0002).

## Run the tests

```bash
scripts/fetch-mls-vectors.sh      # once: pinned, checksum-verified vectors
pnpm --filter @nexus-chat/crypto test
pnpm --filter @nexus-chat/crypto bench   # optional, takes about a minute
```

Coverage must stay at 100% of branches (spec §7.1).

## Patched dependency

`patches/ts-mls@1.6.4.patch` fixes ts-mls issue #858 (a ratchet tree
listing a blank leaf as unmerged was accepted). `src/mls/treeValidation.test.ts`
fails without it. Drop the patch when upstream ships the fix.
