# Nexus Chat

A self-hostable chat platform with servers, text and voice channels, DMs,
voice/video/screen share, and federation between instances. All user
content is end-to-end encrypted with MLS (RFC 9420).

**Status:** early development. The foundation (milestone M0) is in place:
the server starts, migrates its database and answers health checks. It is not
usable as a chat app yet. See [progress](docs/progress.md) and the
[roadmap](docs/roadmap.md).

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
- [Platform support](docs/platform-support.md)
- [Dependencies](docs/dependencies.md)
- [Architecture Decision Records](docs/adr/README.md)

## Security

Please report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE)
