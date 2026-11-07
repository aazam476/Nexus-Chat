// Package httpserver assembles the HTTP handler from every feature's
// handler and wraps it in cross-cutting middleware (logging, panic recovery,
// security headers).
//
// Start reading at [NewHandler]. Adding an API feature means adding its
// handler to [Features] and embedding it in the handlers struct in routes.go.
package httpserver
