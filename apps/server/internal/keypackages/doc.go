// Package keypackages is the MLS key-package directory (RFC 9420 §10;
// docs/e2ee-design.md §3.2): devices upload key packages, and anyone who
// wants to add a user's devices to a group claims one per device.
//
// The server does not run MLS (ADR 0002). It checks only what it can
// without it: the fixed header (MLS 1.0, a key package, ciphersuite
// 0x0001) and the size. Clients verify the signatures and credentials
// before adding anyone. Start reading at service.go.
package keypackages
