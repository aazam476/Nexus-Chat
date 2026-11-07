# ADR 0008: Server address syntax

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

The spec asks for user addresses `@alice@example.com` and server addresses
`server@example.com`, and lets Phase 0 propose an alternative if the server
form collides with email addresses.

## Problem with `name@domain`

- It is exactly an email address. Mail clients, phones, chat apps, and
  browsers auto-link it as `mailto:`, so a shared invite like
  `gaming@example.com` opens an email draft instead of the app.
- Users can't tell from the text whether `support@example.com` is a server
  to join or a mailbox to write to, which invites phishing ("join
  support@yourbank.example").
- In text input, typing a server address next to a user mention is
  ambiguous for the parser (`@gaming@example.com` vs `gaming@example.com`
  differ by one character).

## Options

| Syntax                      | Auto-linked as email?                                                                        | Notes                                                                            |
| --------------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `gaming@example.com` (spec) | Yes                                                                                          | Problems above                                                                   |
| **`+gaming@example.com`**   | Most linkifiers skip a leading `+`, but some still link the part after it, which M5 measures | Parallels `@user@domain`; `+` reads as "group"; not used elsewhere in our syntax |
| `#gaming@example.com`       | No                                                                                           | `#` already means channel mentions, and is a URL fragment                        |
| `~gaming@example.com`       | Varies                                                                                       | Less familiar; `~` is awkward on mobile keyboards                                |
| `gaming:example.com`        | No                                                                                           | Matrix-like; doesn't match our user syntax                                       |

## Decision (proposed)

Canonical server address: **`+gaming@example.com`**. Invite links
(`https://example.com/join/…`) remain the main sharing path, since they
are always clickable. The join box also accepts the bare `gaming@example.com`
form and shows a confirmation ("Join server _Gaming_ on example.com?"), so
the spec's form still works when typed.

## Consequences

- One more character than the spec's form.
- The address parser is a single fuzzed module shared by client and docs
  (`@` = user, `+` = server, `#` = channel).
