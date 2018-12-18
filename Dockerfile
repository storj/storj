FROM golang:1.11.3 as build-env

WORKDIR /storj.io/storj
# add module information for caching
COPY go.mod go.mod
# COPY go.sum go.sum

# get and build dependencies
RUN  go mod download

# setup environment
ENV GOOS=linux GOARCH=amd64

# build dependencies
COPY scripts/build-mod.go scripts/build-mod.go
RUN go run scripts/build-mod.go

# copy source
COPY . .

# build the source
ENV GOBIN=/app
RUN go install ./cmd/...

# Satellite
FROM alpine as storj-satellite
ENV CONF_PATH=/root/.local/share/storj/satellite \
    SATELLITE_ADDR=
EXPOSE 7776/udp 7777 8080
WORKDIR /app
COPY --from=build-env /app/satellite /app/satellite
COPY --from=build-env /storj.io/storj/cmd/satellite/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]

# Storage Node
FROM alpine as storj-storagenode
ENV CONF_PATH=/root/.local/share/storj/storagenode \
    SATELLITE_ADDR=
EXPOSE 7776/udp 7777
WORKDIR /app
COPY --from=build-env /app/storagenode /app/storagenode
COPY --from=build-env /storj.io/storj/cmd/storagenode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]

# Uplink
FROM alpine as storj-uplink
ENV CONF_PATH=/root/.storj/uplink/config.yaml \
    API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7776/udp 7777
WORKDIR /app
COPY --from=build-env /app/uplink /app/uplink
COPY --from=build-env /storj.io/storj/cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
