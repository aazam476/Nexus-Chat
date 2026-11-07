# ADR 0007: Search beyond on-device indexing

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

Message content is E2EE, so the server cannot index it. The spec requires
tiered on-device search (recent window, backfill, encrypted index sync,
desktop as heavy indexer). It also asks for two decisions: (1) whether to
build optional Searchable Symmetric Encryption (SSE), and (2) whether to offer
an unencrypted "server-readable" mode for public announcement channels.

## Findings on SSE

Requirements from the spec: many readers and writers, usable from
TypeScript/React Native and Go without custom crypto, a clear leakage profile,
forward/backward privacy, key rotation tied to MLS epochs, and no
blind-index (keyword HMAC) designs.

- **Constructions.** The literature has dynamic schemes with forward and
  backward privacy (e.g. the Bost et al. line: Σoφoς, Diana/Janus) and some
  multi-client schemes, but most are single-writer (one data owner updates the
  index). Multi-writer, multi-reader dynamic SSE with forward privacy usually
  needs a semi-trusted proxy holding keyword state, or a group key that every
  writer shares, which brings back the problems of a shared key.
- **Libraries.** No maintained, reviewed SSE library usable from both
  TypeScript (React Native) and Go was found. Research code exists (Clusion,
  single-user, Java; a 2026 Rust SDK for RO(SE)², unreviewed). Building SSE
  would mean implementing a scheme from a paper, which the "no custom crypto"
  rule forbids in practice.
- **Leakage.** Even good schemes leak the search pattern (repeated queries)
  and the access pattern (which encrypted messages match). Leakage-abuse
  attacks (Islam-Kuzu-Kantarcioglu, Cash et al. "count" attacks, Zhang et al.
  file-injection) recover queries and keywords from this leakage, and
  file injection is especially easy in a chat app, where any member can post
  messages chosen to probe the index.
- **Rotation.** Tying keys to MLS epochs means re-indexing the channel (or
  re-encrypting all index entries) after every membership change, which for
  large active channels is a cost comparable to the history backfill SSE was
  meant to avoid.

## Decision (proposed)

1. **Do not build SSE for beta.** Keep it as a research item, revisited if a
   reviewed multi-writer library appears. The tiered on-device design plus
   encrypted index sync and desktop heavy indexing covers the "large channel"
   case without new leakage.
2. **Do not offer a server-readable channel mode in beta.** It would create a
   second, non-E2EE code path for messages, add a mode users can confuse with
   the encrypted one, and make the "the server never has plaintext" claim
   conditional. Public announcements can be served by a channel where only
   some roles can post, which needs no plaintext on the server. Revisit after
   beta if users ask for public web archives of channels.

## Consequences

- Search on a brand-new phone over a large server is limited to the recent
  window plus synced index chunks until backfill completes.
- No new metadata leakage beyond index-sync chunk sizes and fetch patterns
  (already in the threat model).
