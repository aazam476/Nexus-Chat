# End-to-end tests

Black-box tests that drive a running Nexus Chat instance through a real
browser and the desktop app (spec §7.2). They use only what a person sees:
labels, roles and test IDs, never the app's code. The release gate
(`scripts/release-gate.sh`) runs them against a fresh `docker compose`
install, over localhost and through each HTTPS reverse proxy.

What they cover today:

- the setup wizard creates the admin, and a second user signs up;
- the admin opens the Admin panel;
- two people sign up, write down their recovery phrases, become friends and
  message each other end-to-end encrypted, live, with typing shown, a
  reply that quotes what it answers and a reaction that names who reacted; a
  second device of one of them restores from the phrase and reads the
  conversation's history;
- a status change on one device reaches the user's other devices live;
- the session survives a reload and ends at logout; failed logins are
  explained;
- the desktop app picks an instance, logs in, has no Node.js in the page,
  and cannot navigate away from the app;
- every screen visited passes axe-core with no serious or critical WCAG
  2.1 AA violations.

## Run them

The instance must be fresh, because the first test runs the setup wizard.
Start a server (see `apps/server/README.md`) with the web build
(`WEB_DIR`), then:

```bash
pnpm --filter @nexus-chat/e2e exec playwright install chromium
NEXUS_E2E_BASE_URL=http://localhost:8080 NEXUS_E2E_SETUP_TOKEN=<token from the logs> \
  pnpm --filter @nexus-chat/e2e e2e
```

`e2e` runs the release gate's projects (`web`, `desktop`); `visual` is separate (below).

| Variable                 | Meaning                                                                   |
| ------------------------ | ------------------------------------------------------------------------- |
| `NEXUS_E2E_BASE_URL`     | The instance, as a browser on this machine reaches it                     |
| `NEXUS_E2E_SETUP_TOKEN`  | The one-time setup token from the server's logs                           |
| `NEXUS_E2E_DESKTOP`      | `1` to run the desktop test (needs a display and the desktop app's build) |
| `NEXUS_E2E_HOST_RULES`   | Chromium host rules, e.g. `MAP nexus.test 127.0.0.1`                      |
| `NEXUS_E2E_TRUSTED_SPKI` | Base64 SHA-256 of a test certificate's public key for Chromium to trust   |
| `NEXUS_E2E_CHROMIUM`     | A Chromium binary to use instead of Playwright's                          |

Trusting the test certificate's key keeps the page a secure context, as
with a real certificate; ignoring certificate errors would not.

Retries are off: a test that fails once fails the run (spec §7.5).

## Visual checks

`specs/visual.spec.ts` shows every screen with the demo, worst-case and
right-to-left data from `tests/fixtures`, in both themes and at 320px
(reflow at 400% zoom, which also covers 200% text), 390px and 1280px. It
fakes the instance at the network boundary (`src/fakeInstance.ts`), so it
needs only the web build. On every screen it checks that no text is cut
off or runs off the screen, nothing scrolls sideways, no controls
overlap, and axe finds nothing serious.

```bash
pnpm --filter @nexus-chat/client build:web
pnpm --filter @nexus-chat/e2e visual
```

Pixel snapshots (`specs/visual.spec.ts-snapshots`) are compared only with
`NEXUS_VISUAL_SNAPSHOTS=1`, inside the Playwright image that CI pins
(`mcr.microsoft.com/playwright:v1.65.0-noble`), because other fonts and
browsers render differently. After an intended change, update them in
that image:

```bash
docker run --rm -v "$PWD:/work" -w /work mcr.microsoft.com/playwright:v1.65.0-noble \
  bash -c "npm install -g pnpm@12.18.0 && pnpm install && pnpm --filter @nexus-chat/api-client build && \
  pnpm --filter @nexus-chat/client build:web && NEXUS_VISUAL_SNAPSHOTS=1 pnpm --filter @nexus-chat/e2e visual --update-snapshots"
```

A failing CI run uploads the differences as the `visual-differences`
artifact.
