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

# The release check runs on the binaries as they came out of the Go build,
# rather than on the signed ones, so that signing cannot influence it. Windows
# is checked on its own, so that signing can wait for its own platform only.
FROM build-tools AS check-windows-binaries
COPY --from=windows_amd64 /* /out/windows_amd64/

WORKDIR /work
COPY scripts/release/check-release-binaries.sh ./
RUN ./check-release-binaries.sh /out && touch /windows-checked

FROM build-tools AS check-binaries
COPY --from=check-windows-binaries /windows-checked /windows-checked
COPY --from=linux_amd64   /* /out/linux_amd64/
COPY --from=linux_arm64   /* /out/linux_arm64/
COPY --from=linux_arm     /* /out/linux_arm/
COPY --from=freebsd_amd64 /* /out/freebsd_amd64/
COPY --from=macos_amd64   /* /out/macos_amd64/
COPY --from=macos_arm64   /* /out/macos_arm64/

WORKDIR /work
COPY scripts/release/check-release-binaries.sh ./
RUN ./check-release-binaries.sh /out && touch /all-checked

# Windows signing: jsign talking to Azure Trusted Signing.
FROM eclipse-temurin:21-jre-alpine@sha256:974b08960c5d96694c780e65b2d5705268ab1e1ca1a0dd0caf4ba6c3fe34d699 AS windows-signer

ADD --checksum=sha256:602a51c3545a6dc4fb99bd2ea7152b26d1345916d0c93ddfbd5936cb735af91c \
    https://github.com/ebourg/jsign/releases/download/7.5/jsign-7.5.jar /jsign.jar

RUN apk add --no-cache curl
COPY scripts/release/sign-artifacts.sh /usr/local/bin/sign-artifacts

FROM windows-signer AS sign-windows-binaries
# Build secrets are not part of the cache key, so without SIGN_ATTEMPT a build
# with credentials would reuse the unsigned artifacts of an earlier one.
ARG SIGN_ATTEMPT
ARG ALLOW_UNSIGNED
# Only sign what passed the release check, a signature over an unreleasable
# build would outlive the failed build itself.
COPY --from=check-windows-binaries /windows-checked /windows-checked
# Everything the windows build produced, so that a new kind of artifact is
# signed as well, or fails the build, rather than silently going out unsigned.
COPY --from=windows_amd64 /* /out/
RUN --mount=type=secret,id=azure_tenant_id \
    --mount=type=secret,id=azure_client_id \
    --mount=type=secret,id=azure_client_secret \
    --mount=type=secret,id=sign_keystore \
    --mount=type=secret,id=sign_alias \
    SIGN_ATTEMPT="${SIGN_ATTEMPT}" ALLOW_UNSIGNED="${ALLOW_UNSIGNED}" sign-artifacts /out/*

# Windows installer: custom action DLL cross-compiled with zig, MSI assembled with wixl (msitools).
FROM build-tools AS build-windows-installer

WORKDIR /work
COPY installer/windows /work/installer/windows
COPY --from=sign-windows-binaries /out/storagenode.exe /out/storagenode-updater.exe /work/bin/

ARG BUILD_VERSION

RUN cd installer/windows/ca && zig build test && zig build --prefix /work/installer/windows
RUN ./installer/windows/build.sh "${BUILD_VERSION}" /work/bin/storagenode.exe /work/bin/storagenode-updater.exe /out/storagenode.msi

FROM windows-signer AS sign-windows-installer
ARG SIGN_ATTEMPT
ARG ALLOW_UNSIGNED
COPY --from=build-windows-installer /out/storagenode.msi /out/
RUN --mount=type=secret,id=azure_tenant_id \
    --mount=type=secret,id=azure_client_id \
    --mount=type=secret,id=azure_client_secret \
    --mount=type=secret,id=sign_keystore \
    --mount=type=secret,id=sign_alias \
    SIGN_ATTEMPT="${SIGN_ATTEMPT}" ALLOW_UNSIGNED="${ALLOW_UNSIGNED}" sign-artifacts /out/storagenode.msi
# The installer is not consumed by another build step, so it can carry the name
# it gets published under.
RUN if [ -e /out/.unsigned ]; then \
        mv /out/storagenode.msi /out/storagenode-installer-unsigned.msi && \
        rm /out/.unsigned; \
    fi

FROM scratch AS export-windows-installer
COPY --from=sign-windows-installer /out/ /

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

# Everything that gets released, assembled into the layout we publish:
# one folder per platform, with the signed Windows artifacts in place.
FROM build-tools AS release-tree
COPY --from=linux_amd64   /* /out/linux_amd64/
COPY --from=linux_arm64   /* /out/linux_arm64/
COPY --from=linux_arm     /* /out/linux_arm/
COPY --from=freebsd_amd64 /* /out/freebsd_amd64/
COPY --from=macos_amd64   /* /out/macos_amd64/
COPY --from=macos_arm64   /* /out/macos_arm64/
# The signed artifacts are the only source for windows, so that an unsigned one
# cannot reach the release layout.
COPY --from=sign-windows-binaries  /out/ /out/windows_amd64/
COPY --from=sign-windows-installer /out/ /out/windows_amd64/

WORKDIR /work
COPY scripts/release/compress-binaries.sh ./
# Only compress what passed the release check.
COPY --from=check-binaries /all-checked /all-checked
# Artifacts that could not be signed are published under a name that says so.
RUN if [ -e /out/windows_amd64/.unsigned ]; then \
        cd /out/windows_amd64 && rm .unsigned && \
        for file in *.exe; do \
            [ -e "$file" ] || continue; \
            mv "$file" "${file%.exe}-unsigned.exe"; \
        done; \
    fi
RUN ./compress-binaries.sh /out

FROM scratch AS export-finalized-binaries
COPY --from=release-tree /out/*.zip /out/sha256sums /
