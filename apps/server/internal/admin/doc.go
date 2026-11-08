// Package admin serves the Admin panel's instance settings that are not
// owned by another feature: the HTTPS certificate (spec §3.3) and the mail
// server (spec §3.5).
//
// Layers:
//   - handler.go: HTTP for every endpoint; each one first checks that the
//     caller is an instance admin
//   - mailsettings.go: validating and saving SMTP settings, and the test email
//
// The certificate itself is validated and served by package tlsconfig.
package admin
