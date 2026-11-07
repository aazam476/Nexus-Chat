// Package tlsconfig serves HTTPS from a certificate the admin provides
// (spec §3.3), either mounted as files or, from milestone M2, uploaded in the
// Admin panel (which writes the same files).
//
//   - certificate.go: checks a certificate before it is used: the key
//     matches, the chain reaches a trusted root, it is currently valid, and
//     it covers the instance's domain.
//   - manager.go: keeps the current certificate, reloads it when the files
//     change without restarting the server, and warns before it expires.
//
// Start reading at [Validate], then [Manager].
package tlsconfig
