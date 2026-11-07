# Nexus Chat

A self-hostable chat platform with servers, text and voice channels, DMs,
voice/video/screen share, and federation between instances. All user
content is end-to-end encrypted with MLS (RFC 9420).

**Status:** early development. An instance can be deployed and set up
(milestone M1), but it is not usable as a chat app yet. See [progress](docs/progress.md) and the
[roadmap](docs/roadmap.md).

## Install

You need Docker with the Compose plugin.

1. Download [`docker-compose.yml`](infra/compose/docker-compose.yml) and
   [`.env.example`](infra/compose/.env.example) into an empty folder.
2. Copy `.env.example` to `.env` and set `ADMIN_EMAIL` and
   `POSTGRES_PASSWORD`.
3. Run `docker compose up -d`.
4. Run `docker compose logs server` and copy the `setup_token`.
5. Open `http://localhost:8080/setup` (or this machine's address) and
   create the admin account with the token.

Voice, encryption in the browser, passkeys and the mobile apps need HTTPS;
see [HTTPS in the self-hosting guide](docs/self-hosting.md#https), which also
covers backups, upgrades and reverse proxies.

## Repository layout

| Path                  | What it is                                                 |
| --------------------- | ---------------------------------------------------------- |
| `apps/server`         | Go server: API, WebSocket, federation, web client          |
| `apps/push-relay`     | Go push relay for APNs and FCM                             |
| `packages/api-client` | TypeScript client generated from the server's OpenAPI file |
| `docs/`               | Design documents, decision records, contributor guides     |
| `scripts/`            | Coverage and license checks used by CI                     |

## Contributing

Start with [CONTRIBUTING.md](CONTRIBUTING.md) and
[getting started](docs/contributing/getting-started.md).

## Design documents

- [Architecture](docs/architecture.md)
- [Threat model](docs/threat-model.md)
- [End-to-end encryption design](docs/e2ee-design.md)
- [Federation protocol](docs/federation-protocol.md)
- [Push notifications](docs/push-notifications.md)
- [Self-hosting guide](docs/self-hosting.md)
- [Platform support](docs/platform-support.md)
- [Dependencies](docs/dependencies.md)
- [Architecture Decision Records](docs/adr/README.md)

## Security

Please report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
