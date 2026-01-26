# syntax=docker/dockerfile:1.4-labs

ARG GO_VERSION="1.25.6"
ARG NODE_VERSION="24.11.1"

# Compile wasm module for satellite ui.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS wasm
WORKDIR /work

RUN apt-get update
RUN apt install -y brotli

COPY go.mod go.sum /work/
COPY web/satellite/wasm/  /work/web/satellite/wasm/

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    ./web/satellite/wasm/release-wasm.sh /out/wasm

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION} AS build
WORKDIR /work/
COPY web/satellite/package*.json /work/
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/satellite/ /work/
RUN --mount=type=cache,target=/root/.npm npm run build
COPY --from=wasm /out/wasm /work/static/wasm

FROM scratch AS ui
COPY --from=build /work/dist   /var/www/html
COPY --from=build /work/static /var/www/static
