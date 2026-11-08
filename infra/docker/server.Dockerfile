# syntax=docker/dockerfile:1
# Image for nexus-chat-server. Build from the repository root:
#   docker build -f infra/docker/server.Dockerfile .

# The web client (spec §2: the server also serves it). The bundle is plain
# JavaScript and the same for every architecture, so it is built once on
# the build machine.
FROM --platform=$BUILDPLATFORM node:26.13.0-trixie-slim@sha256:5adad64f9a1fca2d51972caa65983634c8a774eb300d72b20f7f2a0f60368072 AS web
WORKDIR /src
RUN npm install --global pnpm@12.18.0
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc tsconfig.base.json ./
# The lockfile also refers to the patched dependencies and to the test
# fixtures the client's tests use, so both must be present to install.
COPY patches/ patches/
COPY packages/ packages/
COPY tests/fixtures/ tests/fixtures/
COPY apps/client/ apps/client/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile
RUN pnpm --filter @nexus-chat/api-client build && pnpm --filter @nexus-chat/client build:web

# The build stage runs on the build machine's own architecture and
# cross-compiles, which is much faster than emulating arm64.
FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
# Build the module on its own, without the repository's go.work.
ENV GOWORK=off CGO_ENABLED=0
COPY apps/server/go.mod apps/server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/server/ ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/nexus-chat-server ./cmd/nexus-chat-server
RUN mkdir -p /out/data

# distroless/static contains only CA certificates, time zone data and
# /etc/passwd: no shell or package manager for an attacker to use.
FROM gcr.io/distroless/static-debian13@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f
COPY --from=build /out/nexus-chat-server /nexus-chat-server
COPY --from=build --chown=65532:65532 /out/data /data
COPY --from=web /src/apps/client/dist /web
# The server starts as root only to take ownership of a bind-mounted /data,
# then switches to uid 65532 (internal/platform/privileges).
ENV DATA_DIR=/data WEB_DIR=/web NEXUS_DROP_PRIVILEGES=65532:65532
EXPOSE 8080 8443
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=6 CMD ["/nexus-chat-server", "healthcheck"]
ENTRYPOINT ["/nexus-chat-server"]
