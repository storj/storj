# syntax=docker/dockerfile:1.7-labs

ARG GO_VERSION="1.25.3"
ARG NODE_VERSION="24.11.1"

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-tools

# Install some basic tools
RUN apt-get update && apt install -y build-essential wget xz-utils git brotli ca-certificates curl gnupg zip

# Install Windows resource compiler.
RUN go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@53cb51b8aa6b6b62ab8196e66a766ea7598c67fa

## Install Zig for cross-compilation
ARG BUILDPLATFORM
ARG ZIG_VERSION="0.15.2"

## Install Zig for the specific build platform
RUN case ${BUILDPLATFORM} in \
    "linux/amd64")  ZIG_ARCH=x86_64  ; ZIG_SHA256=02aa270f183da276e5b5920b1dac44a63f1a49e55050ebde3aecc9eb82f93239 ;; \
    "linux/arm64")  ZIG_ARCH=aarch64 ; ZIG_SHA256=958ed7d1e00d0ea76590d27666efbf7a932281b3d7ba0c6b01b0ff26498f667f ;; \
    "linux/arm/v7") ZIG_ARCH=arm     ; ZIG_SHA256=7d8401495065dae45d6249c68d5faf10508f8203c86362ccb698aeaafc66b7cd ;; \
    "linux/386")    ZIG_ARCH=x86     ; ZIG_SHA256=4c6e23f39daa305e274197bfdff0d56ffd1750fc1de226ae10505c0eff52d7a5 ;; \
    esac && \
    wget https://ziglang.org/download/$ZIG_VERSION/zig-$ZIG_ARCH-linux-$ZIG_VERSION.tar.xz && \
    echo "$ZIG_SHA256  zig-$ZIG_ARCH-linux-$ZIG_VERSION.tar.xz" | sha256sum -c - && \
    tar -xf zig-$ZIG_ARCH-linux-$ZIG_VERSION.tar.xz && \
    mv zig-$ZIG_ARCH-linux-$ZIG_VERSION /usr/local/zig && \
    rm zig-$ZIG_ARCH-linux-$ZIG_VERSION.tar.xz
ENV PATH="$PATH:/usr/local/zig"

# Download dependencies in a separate stage, so that we don't start downloading
# them from each separate build-binaries.
FROM build-tools AS download-dependencies

WORKDIR /work
COPY go.mod go.sum ./

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

###
# Building UI components
###

# Just an alias for the node builder image, so we don't have to repeat.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION} AS npm-builder

# Compile wasm module for satellite ui.
FROM download-dependencies AS web-satellite-wasm
WORKDIR /work
# This command gives the list of folders that need to be copied for wasm build:
#   GOOS=js GOARCH=wasm go list -deps ./satellite/console/wasm | grep storj.io/storj
COPY --parents satellite/console/consolewasm satellite/console/wasm /work/
COPY --parents scripts/release/satellite-wasm.sh /work/
RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    ./scripts/release/satellite-wasm.sh /out/wasm

# Satellite UI
FROM npm-builder AS web-satellite
WORKDIR /work/web/satellite
COPY --parents web/satellite/package*.json /work/
RUN --mount=type=cache,target=/root/.npm npm ci
COPY --parents web/satellite/ /work/
RUN --mount=type=cache,target=/root/.npm npm run build
COPY --from=web-satellite-wasm /out/wasm /work/web/satellite/static/wasm

FROM scratch AS web-satellite-export
COPY --from=web-satellite /work/web/satellite/dist   /dist
COPY --from=web-satellite /work/web/satellite/static /static

###
# Building Go binaries
###

FROM download-dependencies AS build-binaries

WORKDIR /work

# Add web dependencies. We only need to add those that are embedded.

COPY . /work/
## Satellite console does not embed the UI.
# COPY --from=web-satellite   /work/web/satellite/dist   /work/web/satellite/dist
COPY --from=web-storagenode /  /work/web/storagenode/dist/
COPY --from=web-multinode   /  /work/web/multinode/dist/

COPY --from=web-satellite-admin        /  /work/satellite/admin/ui/build
COPY --from=web-satellite-admin-legacy /  /work/satellite/admin/legacy/ui/build

ARG GOOS
ARG GOARCH
ARG GO_LDFLAGS

ARG CC
ARG CXX
ARG CGO_ENABLED

ARG BUILD_RELEASE=true

ARG COMPONENTS=./...
ARG VERSION # VERSION is needed for windows-resources

# "set -f" is used to disable globbing to prevent unexpected behavior with glob expansion

RUN if [ "$GOOS" = "windows" ] && [ "$GOARCH" = "amd64" ]; then \
    set -f; \
    VERSION="${VERSION}" ./scripts/release/windows-resources.sh ${COMPONENTS} || exit 1; \
    fi

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    set -f && \
    GOOS=$GOOS GOARCH=$GOARCH \
    CC=$CC \
    CXX=$CXX \
    CGO_ENABLED=$CGO_ENABLED \
    go build -ldflags "${GO_LDFLAGS} -X storj.io/common/version.buildRelease=${BUILD_RELEASE}" -o /out/ ${COMPONENTS}

# Compression is currently disabled to be compatible with old implementations.
# We compress the binaries so that when bake copies out of the docker image, they are smaller.
# RUN ./scripts/release/compress.sh /out/

FROM scratch AS export-binaries
COPY --from=build-binaries /out/* /

FROM scratch AS combine-platforms
COPY --from=linux_amd64 /* /linux_amd64/
COPY --from=linux_arm64 /* /linux_arm64/
COPY --from=linux_arm   /* /linux_arm/

# Some binaries that are necessary for image building.

FROM build-tools AS storj-up-build

WORKDIR /app
RUN git clone --depth 1 https://github.com/storj/storj-up.git /app

RUN mkdir -p /out/linux_amd64 /out/linux_arm64

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=linux GOARCH=amd64 \
    CGO_ENABLED=0 \
    go build -o /out/linux_amd64/storj-up .

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=linux GOARCH=arm64 \
    CGO_ENABLED=0 \
    go build -o /out/linux_arm64/storj-up .

FROM scratch AS storj-up-binaries
COPY --from=storj-up-build /out/linux_amd64 /linux_amd64
COPY --from=storj-up-build /out/linux_arm64 /linux_arm64

FROM build-tools AS delve-build

WORKDIR /app
RUN git clone --depth 1 https://github.com/go-delve/delve.git /app

RUN mkdir -p /out/linux_amd64 /out/linux_arm64

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=linux GOARCH=amd64 \
    CGO_ENABLED=0 \
    go build -o /out/linux_amd64/dlv ./cmd/dlv

RUN \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=linux GOARCH=arm64 \
    CGO_ENABLED=0 \
    go build -o /out/linux_arm64/dlv ./cmd/dlv

FROM scratch AS delve-binaries
COPY --from=delve-build /out/linux_amd64 /linux_amd64
COPY --from=delve-build /out/linux_arm64 /linux_arm64
