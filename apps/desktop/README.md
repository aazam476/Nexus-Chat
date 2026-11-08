# Nexus Chat desktop

The Nexus Chat app for Windows, macOS and Linux: the client's web build in
an Electron window. Desktop features (screen picker, tray, push-to-talk,
`safeStorage`, packaging and updates) arrive in milestone M11.

## Run it

```bash
pnpm install
pnpm --filter @nexus-chat/api-client build
pnpm --filter @nexus-chat/client build:web
pnpm --filter @nexus-chat/desktop start
```

`NEXUS_WEB_DIR` points the app at another web build.

## How it is put together

- `src/main.ts` serves the web build from `app://nexus-chat`, a secure
  origin, so WebCrypto and IndexedDB work as in a browser. Every response
  carries a Content Security Policy that allows scripts only from the
  bundle.
- The window is sandboxed and context-isolated, without Node.js. It can
  only navigate within the app; links open in the system browser, and only
  `https:`, `http:` and `mailto:` links are handed over. Permission
  requests (camera, microphone, notifications) are refused until the
  features that need them arrive.
- `src/preload.ts` is the only bridge to the page. It offers one call,
  `signInWithProvider`, which opens the provider in its own window and
  hands back the fragment of the redirect to `nexuschat://oidc-return`
  (ADR 0016). The shell, not the page, decides the return address.

## Test it

```bash
pnpm --filter @nexus-chat/desktop test
```

`src/files.ts` and `src/navigation.ts` hold every decision the shell makes
and are tested at 100 %. `main.ts` and `preload.ts` only wire those
decisions to Electron and are exercised by starting the app.
