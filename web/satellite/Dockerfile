# syntax=docker/dockerfile:1.4-labs

ARG GO_VERSION="1.25.6"
ARG NODE_VERSION="24.11.1"

# Compile wasm module for satellite ui.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS wasm
WORKDIR /work

RUN apt-get update && apt-get install -y brotli

COPY wasm/go.mod wasm/go.sum /work/wasm/
RUN cd /work/wasm && go mod download

COPY wasm/ /work/wasm/

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    /work/wasm/build-wasm.sh /out/wasm --compress

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION} AS build
WORKDIR /work/
COPY package*.json /work/
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . /work/
RUN --mount=type=cache,target=/root/.npm npm run build
COPY --from=wasm /out/wasm /work/static/wasm

FROM scratch AS ui
COPY --from=build /work/dist   /dist
COPY --from=build /work/static /static
