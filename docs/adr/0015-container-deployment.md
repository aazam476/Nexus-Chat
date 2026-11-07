# ADR 0015: Container images and deployment layout

- Status: Accepted
- Date: 2026-11-07

## Context

Spec §3 requires one `.env` with two variables, `docker compose up -d`, three
containers, generated secrets, and multi-arch signed images. Several details
decide whether that works on the first try.

## Decisions

1. **Base image:** `gcr.io/distroless/static-debian13`, pinned by digest.
   The Go binary is static (`CGO_ENABLED=0`), so the image needs only CA
   certificates and time zone data. No shell means a smaller attack surface
   and fewer CVEs for Trivy to find.
2. **Start as root, then drop privileges.** `DATA_DIR=./data` is a bind
   mount, and Docker creates the folder owned by root. The server starts as
   root, takes ownership of `/data` with `lchown` (so symbolic links cannot
   redirect it), and switches to uid 65532 before opening any port or
   reading any request. Alternatives rejected: running as root throughout
   (unnecessary risk); a separate `chown` init container (a fourth
   container, against §3.2); requiring `chown` by hand (breaks "works on
   first try").
3. **Generated media config.** The server writes `livekit.yaml` and
   `turnserver.conf` with its generated keys into the data folder on every
   start. LiveKit and coturn mount their folders read-only and run as the
   same uid, so the files stay owner-only.
4. **LiveKit single-port UDP** (`rtc.udp_port: 7882`) instead of a
   10,000-port range: Docker publishes large ranges slowly, and admins have
   fewer firewall rules to write. coturn uses 3479 so both TURN servers can
   run together.
5. **coturn refuses private destinations** (`denied-peer-ip` for RFC 1918,
   loopback, link-local, CGNAT and IPv6 ULA ranges), so the relay cannot be
   used to reach the server's internal network.
6. **Healthcheck inside the binary** (`nexus-chat-server healthcheck`),
   since distroless has no `curl`.
7. **Releases:** images are built on the build machine's architecture and
   cross-compiled for arm64 (no emulation). BuildKit attaches the SBOM and
   provenance attestations. Images are signed with keyless cosign through
   GitHub's OIDC identity. Nothing is pushed until Trivy finds no critical
   vulnerabilities and the release gate passes on localhost and behind
   Caddy, Nginx and Traefik.
8. **Trivy is installed from a release binary with a pinned SHA-256**, not
   through a third-party GitHub Action, whose version tags were rewritten to
   credential-stealing code in March 2026 (CVE-2026-33634). Every other
   action is pinned to a full commit SHA for the same reason.

## Consequences

- The server process must never open sockets or read untrusted input before
  dropping privileges; `run` does the drop right after reading `.env`.
- Third-party images (PostgreSQL, LiveKit, coturn, and the proxies used in
  tests) are pinned by digest and updated by Dependabot.
