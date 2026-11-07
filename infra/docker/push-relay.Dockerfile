# syntax=docker/dockerfile:1
# Image for nexus-chat-push-relay. Build from the repository root:
#   docker build -f infra/docker/push-relay.Dockerfile .
FROM --platform=$BUILDPLATFORM golang:1.27.3-trixie@sha256:af1bfc2614ebb1dbd31af514b78740fd678850e76498762b425ecee4c5c6ea80 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0
COPY apps/push-relay/go.mod apps/push-relay/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/push-relay/ ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/nexus-chat-push-relay ./cmd/nexus-chat-push-relay

# The relay keeps no files, so it runs unprivileged from the start.
FROM gcr.io/distroless/static-debian13@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f
COPY --from=build /out/nexus-chat-push-relay /nexus-chat-push-relay
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/nexus-chat-push-relay"]
