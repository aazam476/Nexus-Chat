# ADR 0016: Sign-in with OAuth 2.0 / OpenID Connect providers

- Status: Accepted
- Date: 2026-11-08

## Context

Spec §4.1 asks for custom OAuth 2.0 / OIDC providers configured by the
instance admin (for example a company Keycloak, Authentik, Google or
GitLab). One instance can have several. Clients are the web build, the
Electron app and the mobile apps, which open the provider in a browser
and must get the result back.

Two attacks shape the design:

- **Login CSRF:** an attacker starts a sign-in with their own provider
  account and tricks a victim into opening the callback link, so the
  victim ends up logged in as the attacker and types messages into the
  attacker's account.
- **Link CSRF:** the same trick against "link this provider to my
  account" would attach the attacker's provider identity to the victim's
  account, giving the attacker a way to log in as the victim.

## Options

1. **Plain OAuth 2.0** with provider-specific user-info calls: every
   provider needs custom code.
2. **OIDC only** (authorization code flow, PKCE, `nonce`), any provider
   with discovery (`/.well-known/openid-configuration`): one code path.
   Plain OAuth 2.0 providers without OIDC (for example GitHub) are not
   supported in beta.
3. Let the **client** talk to the provider and send the ID token to the
   server: every client needs provider config, and public clients cannot
   keep a client secret.

## Decision

Option 2, with the server as the OIDC relying party, using
`github.com/coreos/go-oidc/v3` (Apache-2.0) for discovery and ID token
verification and `golang.org/x/oauth2` (BSD-3-Clause) for the code
exchange. Neither library implements new cryptography; ID token
signatures are checked against the provider's published JWKS.

### Flow

1. The client calls `POST /api/v1/auth/oidc/providers/{id}/begin` with a
   `returnTo` address. It receives a `flowId` (kept in memory) and the
   provider's authorization URL, which it opens in a browser.
2. The server stores the flow: state (hashed), nonce, PKCE verifier,
   `returnTo`, and an optional invite code, valid for 10 minutes.
3. The provider redirects to `GET /api/v1/auth/oidc/callback`. The server
   finds the flow by state, exchanges the code with PKCE, verifies the ID
   token (issuer, audience, expiry, signature, nonce) and stores the
   verified claims on the flow. It then redirects to
   `returnTo#flow=<flowId>&code=<completion code>`. Errors redirect with
   `#error=<code>`.
4. The client sends `flowId`, the completion code and a device name to
   `POST /api/v1/auth/oidc/complete`. Only now is a session created (or an
   MFA challenge returned, exactly as for a password login).

The completion code alone is useless: completion also needs the `flowId`,
which only the client that started the flow has. A victim who opens an
attacker's callback link gets a code but not the matching flow, which
defeats login CSRF. Linking uses the same flow with
`POST /api/v1/auth/oidc/providers/{id}/link` and
`POST /api/v1/auth/oidc/link/complete`; both need the session of the user
who started it, which defeats link CSRF.

`returnTo` is either a path on the instance itself (web and desktop) or
the app scheme `nexuschat://` (mobile). Anything else is refused, so the
callback can never redirect to another site.

### Accounts

- A provider identity is the pair (provider, `sub`). Email addresses are
  never used to find an account: a provider that lets users choose their
  own email would otherwise let anyone take over a local account.
- First sign-in with an unknown identity creates an account when the
  provider allows sign-up and the registration mode permits it (open, or
  invite-only with an invite code). The username comes from
  `preferred_username` or the email's local part, made valid and unique.
  The email counts as verified only if the provider says
  `email_verified: true`.
- If the email already belongs to an account, sign-up is refused with
  `email_in_use`; the user logs in another way and links the provider in
  settings.
- Accounts created this way have no password. They can add one through
  password reset, and must keep at least one way to log in: unlinking the
  last provider of an account without a password or passkey is refused.
- An account's second factors still apply after a provider sign-in.

### Configuration

The admin sets an ID (used in URLs), a display name, the issuer URL, the
client ID and secret, extra scopes, and whether sign-up is allowed. The
server checks discovery when the provider is saved. The client secret is
stored encrypted with the settings key. Issuers must use HTTPS, except
loopback addresses for local testing. The Admin panel shows the callback
URL to register at the provider.

## Consequences

- Any standards-compliant OIDC provider works without code changes.
- Plain OAuth 2.0 providers without OIDC need a bridge (for example
  Dex) until a later release adds them.
- Deleting a provider removes its identities. Users who had only that
  way to log in must use password reset.
- The instance needs a public domain (`DOMAIN`) for provider sign-in,
  because the provider must redirect to a fixed callback URL.
