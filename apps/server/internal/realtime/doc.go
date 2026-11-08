// Package realtime runs the WebSocket gateway (docs/realtime-protocol.md)
// and keeps the in-memory state behind it: gateway sessions, their recent
// events for resuming, and presence (ADR 0012).
//
// Layers:
//   - gateway.go and connection.go: one WebSocket each, speaking the
//     protocol
//   - frames.go: the JSON frames, parsed strictly because they come from
//     the network
//   - hub.go and session.go: which devices are connected, events and
//     replay; presence.go: what status each user shows
//   - sweeper.go: forgetting old sessions and expiring custom statuses
//
// Durable data never lives only here: a lost event is recovered by
// reloading over REST, so a restart loses nothing that matters.
//
// Start reading at [Gateway.ServeHTTP].
package realtime
