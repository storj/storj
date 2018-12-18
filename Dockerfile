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
ENV SATELLITE_ADDR=
EXPOSE 7776/udp 7777 8080
WORKDIR /app
COPY --from=build-env /app/satellite /app/satellite
RUN /app/satellite setup
ENTRYPOINT ["/app/satellite", "run"]

# Storage Node
FROM alpine as storj-storagenode
ENV SATELLITE_ADDR=
EXPOSE 7776/udp 7777
WORKDIR /app
COPY --from=build-env /app/storagenode /app/storagenode
RUN /app/storagenode setup
ENTRYPOINT ["/app/storagenode", "run"]

# Uplink
FROM alpine as storj-uplink
ENV API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7776/udp 7777
WORKDIR /app
COPY --from=build-env /app/uplink /app/uplink
RUN /app/uplink setup
ENTRYPOINT ["/app/uplink", "run"]
