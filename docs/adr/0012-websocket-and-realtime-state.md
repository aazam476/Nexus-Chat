# ADR 0012: WebSocket library and in-memory real-time state

- Status: Proposed
- Date: 2026-11-07

## Context

The Go standard library has no WebSocket server. The spec says no
cache/pub-sub for beta: typing and presence live in memory, without
preventing multi-node scaling later.

## Decision

- **`github.com/coder/websocket` v1.8.15** (ISC). It is maintained, built on
  `net/http` and `context`, and has no other dependencies.
  `gorilla/websocket` (BSD) also works, but its API predates `context`,
  which makes cancellation rules harder for new contributors to follow.
- Real-time state lives behind three small interfaces, defined in
  `realtime/`: `EventBus` (publish to a user, conversation, or server),
  `PresenceTracker`, and `TypingTracker`. Beta implementations use maps
  guarded by a `sync.Mutex`, with a clock-driven sweeper goroutine that
  stops when the server's root context is cancelled.
- Durable delivery (messages, membership events) never depends on the bus:
  it is written to Postgres first, so a missed bus event is recovered by the
  resume protocol.

## Consequences

Scaling out later means adding Redis (or Postgres `LISTEN/NOTIFY`)
implementations of the three interfaces, not changing their callers.
