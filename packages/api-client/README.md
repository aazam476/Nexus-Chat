# `@nexus-chat/api-client`

A typed TypeScript client for the Nexus Chat server API. Every operation and
type is generated from
[`apps/server/api/openapi.yaml`](../../apps/server/api/openapi.yaml), so the
client always matches the server.

```ts
import { createApiClient, getReadiness } from "@nexus-chat/api-client";

const client = createApiClient({ baseUrl: "https://chat.example.com" });
const { data, error } = await getReadiness({ client });
```

## Build and regenerate

```bash
pnpm generate   # after the OpenAPI file changes; commit src/generated
pnpm build
```

## Test it

```bash
pnpm test
```

## Read these three files first

1. `src/index.ts`: the only hand-written code, `createApiClient`.
2. `src/index.test.ts`: how the client is used and tested with a fake `fetch`.
3. `openapi-ts.config.ts`: generator settings.
