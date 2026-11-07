// Package contract holds black-box API contract tests (spec §7.2).
//
// The tests build the real server binary, start it against a fresh
// database, call it over HTTP, and validate every response against
// api/openapi.yaml. They never import the server's application packages,
// so they test only what a client can see. The one internal import is the
// databasetest helper, which only creates an empty database for the server.
package contract
