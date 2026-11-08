# Self-Hosting Nexus Chat

This guide covers installing, securing, backing up and upgrading an
instance. You need a Linux machine with Docker (Docker Engine 24 or later
with the Compose plugin), 2 CPU cores and 2 GB of RAM for a small community.

## 1. Install

1. Create a folder and download two files into it:
   [`docker-compose.yml`](../infra/compose/docker-compose.yml) and
   [`.env.example`](../infra/compose/.env.example).
2. Copy `.env.example` to `.env` and set the two required values:

   ```
   ADMIN_EMAIL=you@example.com
   POSTGRES_PASSWORD=a-long-random-password
   ```

3. Start everything:

   ```bash
   docker compose up -d
   ```

4. Read the one-time setup token:

   ```bash
   docker compose logs server | grep setup_token
   ```

5. Open `http://<this machine>:8080/setup`, enter the token, and create the
   first admin account. The token works once and changes on every restart
   until setup is finished, so only someone who can read the server's logs
   can claim the instance.

> **Setting up over plain HTTP:** the setup token and the admin password
> travel unencrypted. Finish setup from the same machine
> (`http://localhost:8080/setup`) or the same trusted local network, or add
> HTTPS first (section 2).

The default installation runs three containers: `server`, `postgres` and
`livekit`. P2P call mode also needs coturn:

```bash
docker compose --profile p2p up -d
```

<a id="https"></a>

## 2. HTTPS

The server starts on plain HTTP. **These features do not work without
HTTPS** (except on `localhost`, which browsers treat as secure for
testing):

- voice and video calls and screen share (browsers block camera,
  microphone and screen capture)
- passkeys and security keys
- end-to-end encryption in the web client (browser crypto APIs need HTTPS)
- Web Push notifications
- the iOS and Android apps (both refuse plain HTTP)
- federation (other instances only talk over HTTPS)

Set `DOMAIN` in `.env` (for example `DOMAIN=chat.example.com`) first, then
choose one of the two options below.

### Option A: install a certificate on the server

Put the certificate **with its full chain** (your certificate followed by
the intermediate certificates) and the private key into the data folder:

```
data/server/tls/cert.pem
data/server/tls/key.pem
```

The server checks that the key matches, the chain is complete and trusted,
the certificate is currently valid and covers `DOMAIN`. It then serves HTTPS
on port 8443 (`HTTPS_PORT`) **without a restart**. Replace the files the
same way when renewing; a broken replacement is rejected and the old
certificate keeps working. The server logs a warning 30, 7 and 1 days before
the certificate expires. (From milestone M2 you can also upload the
certificate in the Admin panel, and expiry warnings appear there and by
email.)

### Option B: a reverse proxy with its own certificate

The server stays on HTTP behind Nginx, Caddy, Traefik or another proxy. Set
`TRUSTED_PROXIES` in `.env` to the proxy's address range, so the server uses
the real client IP (for rate limiting) and knows the connection is HTTPS:

```
TRUSTED_PROXIES=172.16.0.0/12
```

`172.16.0.0/12` covers Docker's default networks, which is right when the
proxy runs in Docker on the same machine. Use `127.0.0.1` if the proxy runs
directly on the host. Never set it wider than your proxies: anyone inside
the range can claim to be any client.

The examples below are tested on every change: the release gate in CI runs
each of them in front of a fresh instance. The proxy must pass WebSocket
upgrades, which the real-time gateway and voice signaling use.

**Caddy** ([`Caddyfile`](../scripts/release-gate/caddy/Caddyfile)), which
also gets a Let's Encrypt certificate automatically:

```
chat.example.com {
	reverse_proxy server:8080
}
```

(The tested file uses `tls internal` because CI has no public domain;
leave that line out.)

**Nginx** ([`nginx.conf`](../scripts/release-gate/nginx/nginx.conf)):

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 443 ssl;
    server_name chat.example.com;
    ssl_certificate     /etc/nginx/certs/cert.pem;
    ssl_certificate_key /etc/nginx/certs/key.pem;
    client_max_body_size 30m;

    location / {
        proxy_pass http://server:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 1h;
    }
}
```

**Traefik** ([`dynamic.yml`](../scripts/release-gate/traefik/dynamic.yml)),
using the file provider:

```yaml
http:
  routers:
    nexus-chat:
      rule: "Host(`chat.example.com`)"
      entryPoints: [websecure]
      service: nexus-chat
      tls: {}
  services:
    nexus-chat:
      loadBalancer:
        servers:
          - url: "http://server:8080"
```

If the proxy runs outside Docker, replace `server:8080` with
`127.0.0.1:8080` (or your `HTTP_PORT`).

**Voice signaling.** LiveKit's signaling WebSocket will be served through
the chat server itself (from milestone M9), so the same certificate and
proxy rule cover it and no extra proxy configuration is needed. LiveKit's
media ports (section 3) never go through the proxy.

## 3. Sign-in providers (OpenID Connect)

Users can log in with your organization's identity provider (Keycloak,
Authentik, Google Workspace, GitLab and any other OpenID Connect provider).
Provider sign-in needs `DOMAIN`, because the provider sends users back to a
fixed address. At the provider, create a client with:

- **Redirect URI:** `https://<your domain>/api/v1/auth/oidc/callback` (the
  Admin panel shows the exact address)
- **Grant type:** authorization code; PKCE (S256) is always used
- **Scopes:** `openid email profile`

Then add it in the Admin panel with its issuer URL, client ID and secret.
The server checks the issuer's `/.well-known/openid-configuration` when you
save. Turn on **Allow sign-up** to create accounts on first sign-in (the
registration mode still applies). Users with an existing account link the
provider in their settings; accounts are never matched by email address.
See [ADR 0016](adr/0016-oidc-sign-in.md) for the design.

## 4. Ports and firewall

| Port                | Protocol    | Used by                                          | Open to the internet?                       |
| ------------------- | ----------- | ------------------------------------------------ | ------------------------------------------- |
| 8080 (`HTTP_PORT`)  | TCP         | Web client, API, WebSocket                       | Yes, unless only a reverse proxy reaches it |
| 8443 (`HTTPS_PORT`) | TCP         | HTTPS when a certificate is installed (option A) | Yes, with option A                          |
| 7881                | TCP         | WebRTC media over TCP (fallback)                 | Yes, for voice                              |
| 7882                | UDP         | WebRTC media (all calls share this one port)     | Yes, for voice                              |
| 3478                | UDP         | LiveKit's built-in TURN relay                    | Yes, for voice                              |
| 3479                | UDP and TCP | coturn (only with `--profile p2p`)               | Yes, for P2P calls                          |
| 49160–49200         | UDP         | coturn relay ports (only with `--profile p2p`)   | Yes, for P2P calls                          |

PostgreSQL is never exposed outside Docker.

## 5. Backups

Everything is in two places: the database and the data folder
(`DATA_DIR`, by default `./data`). The data folder contains the instance's
keys; **keep backups private.** Message content is end-to-end encrypted, so
a backup holds only ciphertext, but it does contain metadata (see the
[threat model](threat-model.md)). Server credentials kept in the database
(the SMTP password, sign-in provider secrets, TOTP secrets) are encrypted
with keys in `DATA_DIR/secrets`, so store the database dump and the data
folder backup in different places.

```bash
docker compose exec -T postgres pg_dump -U nexuschat --format=custom nexuschat > nexus-chat.dump
```

```bash
tar -czf nexus-chat-data.tar.gz --exclude=./data/postgres ./data
```

Encrypting the disk the data folder lives on is recommended.

## 6. Restore

On a fresh machine with the same `docker-compose.yml` and `.env`:

```bash
tar -xzf nexus-chat-data.tar.gz
```

```bash
docker compose up -d postgres
```

```bash
docker compose exec -T postgres pg_restore -U nexuschat -d nexuschat --clean --if-exists < nexus-chat.dump
```

```bash
docker compose up -d
```

## 7. Upgrades

```bash
docker compose pull && docker compose up -d
```

Database migrations run automatically when the new server starts. No manual
SQL is ever needed. Take a backup before upgrading.

## 8. Troubleshooting

| Symptom                                               | Likely cause and fix                                                                                                                                       |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `required variable ADMIN_EMAIL is missing a value`    | `.env` is missing or not in the same folder as `docker-compose.yml`                                                                                        |
| Server restarts with `invalid configuration`          | The log line lists every bad value in `.env`; fix them and run `docker compose up -d` again                                                                |
| `connecting to database` errors                       | `POSTGRES_PASSWORD` changed after the first start. The database keeps its original password; change it back, or reset the database (this deletes all data) |
| No setup token in the logs                            | Setup is already complete. The token only exists until the first admin is created                                                                          |
| HTTPS port refuses connections or fails the handshake | No valid certificate is installed. `docker compose logs server` says why the certificate was rejected (for example, a missing intermediate certificate)    |
| Rate limits hit everyone at once behind a proxy       | `TRUSTED_PROXIES` is not set, so all requests appear to come from the proxy                                                                                |
| Calls connect but there is no audio or video          | UDP 7882 and 3478 are blocked by a firewall, or the machine is behind NAT without those ports forwarded                                                    |
| `livekit` keeps restarting                            | It starts only after the server is healthy and reads `data/server/livekit/livekit.yaml`; check `docker compose logs livekit`                               |
