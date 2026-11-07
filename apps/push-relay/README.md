# Nexus Chat push relay (`apps/push-relay`)

The service the project publisher runs to forward encrypted push payloads
from instances to APNs and FCM. It never sees plaintext. The design is in
[docs/push-notifications.md](../../docs/push-notifications.md).

Today (milestone M0) it serves only `/healthz`. The relay protocol is built
in milestone M8.

## Run it

```bash
go run ./cmd/nexus-chat-push-relay
```

It listens on port 8090 (set `PORT` to change it).

## Test it

```bash
../../scripts/go-coverage.sh .
```

## Read these three files first

1. `cmd/nexus-chat-push-relay/main.go`: startup and shutdown.
2. `internal/relayconfig/config.go`: the settings it reads.
3. [docs/push-notifications.md](../../docs/push-notifications.md): what the
   relay will do.
