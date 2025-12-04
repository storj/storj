ARG GO_VERSION=1.24.7

FROM storjlabs/golang:${GO_VERSION} AS wasm
WORKDIR /go/src/storj.io/storj
ENV GOOS=js
ENV GOARM=6
ENV CGO_ENABLED=1
ENV TAG=docker
COPY go.mod go.sum ./
COPY satellite/console/wasm satellite/console/wasm
COPY scripts/build-wasm.sh scripts/build-wasm.sh
COPY satellite/console/consolewasm satellite/console/consolewasm
RUN scripts/build-wasm.sh


FROM debian:bookworm AS build
ENV NODE_MAJOR=20
RUN apt-get update && apt-get install -y ca-certificates curl gnupg
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
ENV NODE_MAJOR=24
RUN echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" | tee /etc/apt/sources.list.d/nodesource.list
RUN apt-get update && DEBIAN_FRONTEND="noninteractive" apt-get install -y brotli unzip libuv1-dev libjson-c-dev nettle-dev nodejs git-restore-mtime
ADD web/satellite /work
WORKDIR /work
RUN npm ci --prefer-offline --no-audit --loglevel verbose && npm run build


FROM scratch as ui
COPY --from=build /work/dist /var/www/html
COPY --from=build /work/static /var/www/static
COPY --from=wasm /go/src/storj.io/storj/release/docker/wasm /var/www/static/wasm
