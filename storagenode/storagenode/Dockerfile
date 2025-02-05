ARG DOCKER_PLATFORM
ARG DOCKER_ARCH
ARG GO_DOCKER_PLATFORM
ARG GO_VERSION=1.23.5
ARG BUILD_VERSION

FROM --platform=${DOCKER_PLATFORM:-linux/amd64} ${DOCKER_ARCH:-amd64}/debian:bookworm-slim as ca-cert
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates
RUN update-ca-certificates

FROM --platform=${GO_DOCKER_PLATFORM:-linux/amd64} golang:${GO_VERSION}-bookworm AS builder
ARG CGO_ENABLED=1
RUN echo "Building storagenode version ${BUILD_VERSION}"
COPY go.mod go.sum ./
RUN go mod download
COPY ./storagenode ./storagenode
COPY ./web/storagenode ./web/storagenode
COPY ./shared ./shared
COPY ./private ./private
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" \
    go install -ldflags "-X storj.io/common/version.buildRelease=true \
    -X storj.io/common/version.buildVersion=${BUILD_VERSION} \
    -X storj.io/common/version.buildTimestamp=$(date +%s) \
    -X storj.io/common/version.buildRelease=true" ./storagenode/storagenode

FROM --platform=${DOCKER_PLATFORM:-linux/amd64} ${DOCKER_ARCH:-amd64}/debian:bookworm-slim
COPY --from=ca-cert /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/bin/storagenode /app/bin/storagenode
WORKDIR /app
