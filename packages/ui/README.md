# @nexus-chat/ui

Shared, accessible components, theme tokens and motion constants for the
Nexus Chat client (ADR 0017).

- `tokens.ts`: every color, size and spacing value, for light and dark.
  `tokens.test.ts` checks every text and control color pair against WCAG
  2.1 AA contrast.
- `motion.ts`: every animation value, named. Components never write a
  duration or curve inline; reduced motion swaps movement for a fade.
- Components: `Screen`, `Stack`, `Text`, `Heading`, `Button`, `TextField`,
  `Banner`, `Link`. Each sets its accessible role and name, shows a focus
  ring, keeps a 44-point touch target and follows the user's text size.

## Test it

```bash
pnpm --filter @nexus-chat/ui test
```

## Read first

1. `src/tokens.ts`
2. `src/Button.tsx`: focus, press feedback and reduced motion in one place.
3. `src/TextField.tsx`: labels, hints and errors linked for screen readers.
