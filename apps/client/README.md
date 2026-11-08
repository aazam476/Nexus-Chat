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
