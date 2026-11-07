# syntax=docker/dockerfile:1
# Image for nexus-chat-server. Build from the repository root:
#   docker build -f infra/docker/server.Dockerfile .

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
# The server starts as root only to take ownership of a bind-mounted /data,
# then switches to uid 65532 (internal/platform/privileges).
ENV DATA_DIR=/data NEXUS_DROP_PRIVILEGES=65532:65532
EXPOSE 8080 8443
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=6 CMD ["/nexus-chat-server", "healthcheck"]
ENTRYPOINT ["/nexus-chat-server"]
