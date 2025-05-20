ARG DOCKER_PLATFORM
ARG DOCKER_ARCH
ARG GO_DOCKER_PLATFORM
ARG GO_VERSION=1.24.2

FROM --platform=${DOCKER_PLATFORM:-linux/amd64} ${DOCKER_ARCH:-amd64}/debian:bookworm-slim AS base
RUN apt-get update

FROM base AS ca-cert
RUN apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM --platform=${GO_DOCKER_PLATFORM:-linux/amd64} golang:${GO_VERSION}-bookworm AS builder
ARG CGO_ENABLED=1
ARG BUILD_VERSION
ARG BUILD_DATE
ARG BUILD_COMMIT
COPY go.mod go.sum ./
RUN go mod download
COPY ./storagenode ./storagenode
COPY ./web/storagenode ./web/storagenode
COPY ./shared ./shared
COPY ./private ./private
COPY ./certificate ./certificate
COPY ./cmd ./cmd
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" \
    go install -ldflags "-X storj.io/common/version.buildRelease=true \
      -X storj.io/common/version.buildVersion=$BUILD_VERSION \
      -X storj.io/common/version.buildCommitHash=$BUILD_COMMIT \
      -X storj.io/common/version.buildTimestamp=$BUILD_DATE" \
      ./storagenode/storagenode ./cmd/identity

FROM base AS build
RUN apt-get install -y graphviz
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/storagenode /app/storagenode
COPY --from=builder /go/bin/identity /usr/local/bin/identity
WORKDIR /app
