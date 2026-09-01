# syntax=docker/dockerfile:1.7-labs

ARG GO_VERSION="1.26.6"

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-tools

# Install some basic tools
RUN apt-get update && apt install -y build-essential wget xz-utils git brotli ca-certificates curl gnupg zip wixl

# Install Windows resource compiler.
RUN go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@53cb51b8aa6b6b62ab8196e66a766ea7598c67fa

## Install Zig for cross-compilation
ARG BUILDPLATFORM
ARG ZIG_VERSION="0.16.0"

## Install Zig for the specific build platform
RUN case ${BUILDPLATFORM} in \
    "linux/amd64")  ZIG_ARCH=x86_64  ; ZIG_SHA256=70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00 ;; \
    "linux/arm64")  ZIG_ARCH=aarch64 ; ZIG_SHA256=ea4b09bfb22ec6f6c6ceac57ab63efb6b46e17ab08d21f69f3a48b38e1534f17 ;; \
    "linux/arm/v7") ZIG_ARCH=arm     ; ZIG_SHA256=f85116bf2f9189bb6ae280c7f92f03b89c2551a88e17881c0c2df86bf4e42c50 ;; \
    "linux/386")    ZIG_ARCH=x86     ; ZIG_SHA256=4e34e279a9f856358de420490b531974c3d37f8f3707eef9f0342e92c14c301f ;; \
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

ARG GOOS
ARG GOARCH
ARG GO_LDFLAGS

ARG CC
ARG CXX
ARG CGO_ENABLED

ARG BUILD_VERSION # BUILD_VERSION is needed for windows-resources
ARG BUILD_RELEASE=true

ARG COMPONENTS=./...

# "set -f" is used to disable globbing to prevent unexpected behavior with glob expansion

RUN if [ "$GOOS" = "windows" ] && [ "$GOARCH" = "amd64" ]; then \
    set -f; \
    BUILD_VERSION="${BUILD_VERSION}" ./scripts/release/windows-resources.sh ${COMPONENTS} || exit 1; \
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

# Windows installer: custom action DLL cross-compiled with zig, MSI assembled with wixl (msitools).
FROM build-tools AS build-windows-installer

WORKDIR /work
COPY installer/windows /work/installer/windows
COPY --from=windows_amd64 /storagenode.exe /storagenode-updater.exe /work/bin/

ARG BUILD_VERSION

RUN cd installer/windows/ca && zig build test && zig build --prefix /work/installer/windows
RUN ./installer/windows/build.sh "${BUILD_VERSION}" /work/bin/storagenode.exe /work/bin/storagenode-updater.exe /out/storagenode.msi

FROM scratch AS export-windows-installer
COPY --from=build-windows-installer /out/* /

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
