# syntax=docker/dockerfile:1.4-labs
ARG GO_VERSION="1.25.3"
ARG NODE_VERSION="24.11.1"

# ca-cert downloads certificates in a separate stage to
# avoid needing to download it for different target platforms.
# Similarly, it avoids the apt cache.
FROM --platform=$BUILDPLATFORM debian:trixie-slim AS ca-cert

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

# Install storj-up for local/dev runs
FROM --platform=$TARGETPLATFORM golang:$GO_VERSION AS storjup
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod  \
    CGO_ENABLED=0 go install storj.io/storj-up@latest

# test identities for quick-start (pinned to amd64 as base image lacks arm/v7)
FROM --platform=linux/amd64 img.dev.storj.io/storjup/base:20230607-1 AS identities

# Install dlv for debugging
FROM --platform=$TARGETPLATFORM golang:$GO_VERSION AS dlv
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod  \
    go install github.com/go-delve/delve/cmd/dlv@latest

FROM debian:trixie-slim

ARG TARGETOS
ARG TARGETARCH

ENV CONF_PATH=/root/.local/share/storj/satellite \
    STORJ_CONSOLE_STATIC_DIR=/app \
    STORJ_MAIL_TEMPLATE_PATH=/app/static/emails \
    STORJ_CONSOLE_ADDRESS=0.0.0.0:10100
ENV PATH=$PATH:/app

EXPOSE 7777
EXPOSE 10100

WORKDIR /app

COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=storjup /go/bin/storj-up /usr/local/bin/storj-up
COPY --from=dlv     /go/bin/dlv      /usr/local/bin/dlv

COPY --from=identities /var/lib/storj/identities /var/lib/storj/identities

COPY --from=binaries /${TARGETOS}_${TARGETARCH}/satellite /app/satellite

COPY --from=ui /dist   /app/dist
COPY --from=ui /static /app/static

COPY cmd/satellite/entrypoint   /app/entrypoint

ENTRYPOINT ["/app/entrypoint"]
