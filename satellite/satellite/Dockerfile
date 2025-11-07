ARG GO_VERSION=1.24.7

FROM debian:bookworm-slim AS base
RUN apt-get update

FROM base AS ca-cert
RUN apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM golang:${GO_VERSION}-bookworm AS builder
COPY go.mod go.sum ./
RUN go mod download
COPY ./satellite ./satellite
COPY ./shared ./shared
COPY ./private ./private
ENV GOCACHE=/root/.cache/go-build
ARG CGO_ENABLED=1
ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_COMMIT
RUN --mount=type=cache,target="/root/.cache/go-build" \
    go install -ldflags "-X storj.io/common/version.buildRelease=true \
      -X storj.io/common/version.buildVersion=$BUILD_VERSION \
      -X storj.io/common/version.buildCommitHash=$BUILD_COMMIT \
      -X storj.io/common/version.buildTimestamp=$BUILD_DATE" ./satellite/satellite

FROM base AS build
ENV STORJ_CONSOLE_STATIC_DIR=/app
ENV STORJ_MAIL_TEMPLATE_PATH=/app/static/emails
RUN apt-get install -y graphviz
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/satellite /app/satellite
COPY --from=webui /var/www/html /app/dist
COPY --from=webui /var/www/static /app/static
WORKDIR /app


