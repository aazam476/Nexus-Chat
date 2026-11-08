# Test fixtures

Data every screen is checked against (code standards §8.5, worst-case
data rule), as the API returns it:

- `demo`: the kind data the screens were designed with.
- `worstCase`: names at the server's length limits, Cyrillic and Arabic
  text, emoji-only and one-letter names, unbreakable email addresses, a
  certificate with 15 names and a long issuer that expires in 1 day, and
  long validation messages, on an instance reached over plain HTTP.
- `rightToLeft`: right-to-left names everywhere, and the shortest names.

Component tests in `apps/client` (`src/__tests__/worstCase.test.tsx`) and
the visual checks in `tests/e2e` (`specs/visual.spec.ts`) use the same
data. When a screen gains a field, add its worst case here.

## Files to read first

1. `src/index.ts`: the datasets and why each value is what it is.
2. `../e2e/src/fakeInstance.ts`: how the visual checks serve them.
3. `../../apps/client/src/__tests__/worstCase.test.tsx`: the component tests.

There is nothing to run here; `pnpm --filter @nexus-chat/fixtures typecheck`
checks the data against the API types.
