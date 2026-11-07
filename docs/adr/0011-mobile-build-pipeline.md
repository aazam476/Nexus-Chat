# ADR 0011: Mobile build and submit pipeline

- Status: Proposed
- Date: 2026-11-07

## Context

The spec asks Phase 0 to choose EAS (Expo's cloud build and submit service)
or an equivalent CI pipeline, considering free-tier cost.

## Findings

- EAS Free plan (November 2026): about 15 Android and 15 iOS builds per
  month, at low priority, after which builds wait until the next month.
  Paid plans start at $19/month; extra builds cost roughly $1–4 each.
- `eas build --local` runs the same build steps on our own runner at no
  EAS cost, and `eas-cli` is MIT.
- GitHub Actions provides macOS and Linux runners. For a public repository,
  standard runners are free.

## Decision

Build in **GitHub Actions with `eas build --local`**: Linux runners for
Android and macOS runners for iOS. Upload with `eas submit` (or fastlane if
`eas submit` turns out to require a paid plan). Signing credentials are kept
as GitHub encrypted secrets in a protected `release` environment.

## Consequences

- No monthly build cap and no dependency on a paid plan. Release CI uploads to
  TestFlight and Play internal testing.
- We maintain the runner setup (Xcode version pinning), which EAS cloud would
  otherwise do for us.
- If the repository becomes private, macOS runner minutes cost money, and
  this ADR should be revisited.
