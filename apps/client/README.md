# Nexus Chat client

The Nexus Chat app for iOS, Android and the web, built with Expo and Expo
Router. The same code is the web build every instance serves, and the
desktop app wraps that web build.

## Run it

```bash
pnpm install
pnpm --filter @nexus-chat/api-client build
pnpm --filter @nexus-chat/client web
```

Open the address Expo prints and enter your instance's address, for
example `localhost:8080` for a server started from `apps/server`. The
native apps need a development build (`npx expo run:ios` or
`npx expo run:android`), not Expo Go.

`pnpm --filter @nexus-chat/client build:web` produces the web build in
`dist/`, which the server serves when `WEB_DIR` points at it.

## Test it

```bash
pnpm --filter @nexus-chat/client test
```

Screens are tested with React Native Testing Library against a fake
instance (`src/testing/fakeServer.ts`), so tests exercise the real API
client. Tests live under `src/`, because every file in `app/` is a route.

## Read first

1. `app/_layout.tsx`: the providers every screen gets.
2. `src/connection/useConnectionState.ts`: which instance, which session.
3. `app/login.tsx` with `src/auth/useLogin.ts`: how a screen keeps its
   logic in hooks.

Strings live in `locales/en.json`; ESLint rejects text written into JSX.
Platform differences live in sibling files (`*.web.ts`, `*.native.ts`)
that the bundler picks.

## Mobile end-to-end tests (Maestro)

`.maestro/` holds the flows a person would follow on a phone: the setup
wizard on a fresh instance, a wrong password, logging in and out, and
signing up. CI builds the release APK, starts a fresh instance, and runs
them on an Android emulator (`scripts/maestro-android.sh`).

Android refuses plain-HTTP connections. The test instance in CI has no
certificate, so CI builds with `NEXUS_E2E_CLEARTEXT=1`, which
`app.config.ts` turns into permission for plain HTTP. Store builds never
set it.

To run them yourself against an emulator and an instance on port 8080:

```bash
NEXUS_E2E_CLEARTEXT=1 npx expo run:android --variant release
maestro test .maestro -e INSTANCE=http://10.0.2.2:8080 -e SETUP_TOKEN=<token> -e PASSWORD="maestro admin password"
```
