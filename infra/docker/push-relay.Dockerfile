# syntax=docker/dockerfile:1
# Image for nexus-chat-push-relay. Build from the repository root:
#   docker build -f infra/docker/push-relay.Dockerfile .
FROM --platform=$BUILDPLATFORM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS build
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
FROM gcr.io/distroless/static-debian13@sha256:1a64f46364620465ef9e9a8f5413757dfdf22695e050bbfd7303eeaf17a59383
COPY --from=build /out/nexus-chat-push-relay /nexus-chat-push-relay
USER 65532:65532
EXPOSE 8090
ENTRYPOINT ["/nexus-chat-push-relay"]
