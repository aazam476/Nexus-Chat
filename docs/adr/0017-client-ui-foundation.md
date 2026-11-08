# ADR 0017: Client UI foundation

- Status: Accepted
- Date: 2026-11-08

## Context

One Expo codebase serves iOS, Android, the web build that every instance
hosts, and the Electron app (spec §2). Every screen must meet WCAG 2.1 AA
(spec §4.12), follow the system theme, support 200 % font scaling, respect
reduced motion, and keep every string translatable (spec §4.13). The code
standards ask for code a newcomer can read.

## Options

1. **A component library**: React Native Paper (MIT, Material Design),
   Tamagui (MIT, needs its own compiler for good performance), gluestack UI
   (MIT, built on NativeWind and Tailwind).
2. **Our own small primitives** on React Native's core components (`View`,
   `Text`, `Pressable`, `TextInput`), which `react-native-web` renders as
   accessible HTML.

## Decision

Option 2, in `packages/ui`:

- **Primitives:** `Screen`, `Stack`, `Text`, `Heading`, `Button`,
  `TextField`, `Banner`, `Link`. Each sets its accessibility role and label,
  a visible focus ring on web, and a 44×44 point minimum touch target.
- **Theme tokens** (`tokens.ts`): colors for light and dark, spacing,
  radii and type sizes. Every text/background pair is checked for ≥ 4.5:1
  contrast, and every control outline for ≥ 3:1, by a unit test.
- **Motion** (`motion.ts`): every duration, curve and spring as a named
  constant, and a `useMotion` hook that returns no motion when the system
  asks for reduced motion. Animations use `react-native-reanimated`
  (MIT), the standard for animations that stay smooth on every platform.
- **Styling** with `StyleSheet`; no utility-class system, so styles read as
  plain objects.
- **Translations:** `i18next` with `react-i18next` (both MIT). Strings live
  in `apps/client/locales/en.json`; ESLint rejects string literals inside
  JSX text so a hard-coded string cannot slip in.
- **Navigation:** Expo Router, file-based, so a screen's address is where
  its file is.
- **Session storage:** the token is kept with `expo-secure-store` (Keychain
  / Keystore) on iOS and Android. On web it is kept in IndexedDB,
  encrypted with a non-extractable AES-GCM key from WebCrypto (spec §5.3),
  so script on another origin or a copied browser profile cannot read it
  as text. Electron uses the same web storage; `safeStorage` arrives with
  the desktop features in M11.

## Consequences

- No third-party look to override and no compiler step; we own a small
  set of components and must build new ones as screens need them.
- Accessibility is decided in one place per component, and tested there.
- If a library becomes worth it later, the primitives' props are the
  boundary to swap behind.
