// Package health implements the liveness (/healthz) and readiness (/readyz)
// probes that Docker healthchecks call (spec §3.7).
//
// Liveness only proves the process answers HTTP. Readiness also checks every
// dependency, so an orchestrator stops sending traffic while the database is
// unreachable. Start reading at [Service.Readiness]; handler.go adapts the
// result to the generated OpenAPI types.
package health
