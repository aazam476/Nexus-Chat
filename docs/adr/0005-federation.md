# ADR 0005: Federation model and server-to-server authentication

- Status: **Proposed [REVIEW]**
- Date: 2026-11-07

## Context

The spec recommends a home-hosted model and asks for a choice between HTTP
Message Signatures (RFC 9421) and mTLS.

## Model options

| Option                                      | Description                                                                                       | Verdict                                                                                                                                           |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Home-hosted**                             | A server lives on one instance, which is its delivery service. Remote users' home instances relay | **Accept.** Matches MLS's single-delivery-service design, gives one authority for commit ordering and permissions, and keeps storage in one place |
| Full replication                            | Every participating instance stores the room (Matrix-style)                                       | Reject. Needs a state-resolution algorithm, multiplies storage, and conflicts with "one DS orders MLS commits"                                    |
| Client connects to remote instance directly | Devices log in to every instance                                                                  | Reject. Leaks device IPs to every instance and needs remote credentials                                                                           |

The cost of home-hosted is availability: if the hosting instance is down,
its servers are unavailable. That is accepted for beta and documented.

## Authentication options

|                                                     | RFC 9421 HTTP signatures                                     | mTLS                                                                                                                |
| --------------------------------------------------- | ------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- |
| Works behind an admin's reverse proxy               | Yes. Signatures travel in headers end to end                 | No. The proxy terminates TLS, so client certificates would need custom forwarding, which most basic setups can't do |
| Works with an uploaded cert or a proxy alike        | Yes                                                          | Only with an uploaded cert                                                                                          |
| Key distribution                                    | Ed25519 keys in `/.well-known/nexus-chat`, served over HTTPS | Needs a federation CA or pinning                                                                                    |
| Per-request non-repudiation (for audit and reports) | Yes                                                          | No                                                                                                                  |
| Library                                             | `github.com/yaronf/httpsign` v0.6.2 (Apache-2.0)             | Go standard library                                                                                                 |

## Decision (proposed)

Home-hosted model, with **RFC 9421 signatures over HTTPS**, using Ed25519
keys published in discovery and `Content-Digest` (RFC 9530). The full
profile is in [`federation-protocol.md`](../federation-protocol.md).
TLS still protects the transport, and federation is HTTPS-only.

## Consequences

- The instance's federation key becomes a long-term identity. Rotation is
  supported through overlapping keys in discovery.
- `httpsign` is pre-1.0. It is wrapped behind a small `federation/signing`
  interface, and our profile is pinned with RFC 9421 Appendix B test vectors.
