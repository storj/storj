ARG DOCKER_ARCH
# Satellite UI static asset generation
FROM node:24.11.1 as ui
WORKDIR /app
COPY web/satellite/ /app
# Need to clean up (or ignore) local folders like node_modules, etc...
RUN npm install
RUN npm run build

# Fetch ca-certificates file for arch independent builds below
FROM debian:buster-slim as ca-cert
RUN echo "deb http://archive.debian.org/debian buster main\ndeb http://archive.debian.org/debian-security buster/updates main\ndeb http://archive.debian.org/debian buster-updates main " >/etc/apt/sources.list
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

# Install storj-up helper (for local/dev runs)
FROM --platform=$TARGETPLATFORM golang:1.24 AS storjup
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod  \
    go install storj.io/storj-up@latest

# Install dlv (for local/dev runs)
FROM --platform=$TARGETPLATFORM golang:1.24 AS dlv
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod  \
    go install github.com/go-delve/delve/cmd/dlv@latest

FROM ${DOCKER_ARCH:-amd64}/debian:buster-slim
ARG TAG
ARG GOARCH
ENV GOARCH ${GOARCH}
ENV CONF_PATH=/root/.local/share/storj/satellite \
    STORJ_CONSOLE_STATIC_DIR=/app \
    STORJ_MAIL_TEMPLATE_PATH=/app/static/emails \
    STORJ_CONSOLE_ADDRESS=0.0.0.0:10100
ENV PATH=$PATH:/app
EXPOSE 7777
EXPOSE 10100
WORKDIR /app
COPY --from=ui /app/static /app/static
COPY --from=ui /app/dist /app/dist
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY release/${TAG}/wasm /app/static/wasm
COPY release/${TAG}/satellite_linux_${GOARCH:-amd64} /app/satellite
COPY --from=storjup /go/bin/storj-up /usr/local/bin/storj-up
COPY --from=dlv /go/bin/dlv /usr/local/bin/dlv
# test identities for quick-start
COPY --from=img.dev.storj.io/storjup/base:20230607-1 /var/lib/storj/identities /var/lib/storj/identities
COPY cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
