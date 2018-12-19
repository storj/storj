FROM golang:1.11.4-stretch AS compiler

#TODO: use storjlabs/golang as base

RUN apt update && apt install time

###
# Setup build environment
FROM compiler AS build

###
# Download modules

WORKDIR /storj.io/storj
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

###
# Compile large dependencies up-front for caching
COPY scripts/deps.go scripts/deps.go

#TODO: specify -race from Makefile rather than always
ARG BUILDFLAGS="-race"
RUN go build ${BUILDFLAGS} -v ./scripts/deps.go

###
# Compile binaries
COPY . .
RUN go install ${BUILDFLAGS} -v ./cmd/storagenode ./cmd/satellite ./cmd/gateway

###
# Setup binaries base image
FROM alpine

COPY --from=build /go/bin/ /app/
COPY cmd/gateway/entrypoint     /entrypoint/gateway
COPY cmd/satellite/entrypoint   /entrypoint/satellite
COPY cmd/storagenode/entrypoint /entrypoint/storagenode
COPY cmd/uplink/entrypoint      /entrypoint/uplink
WORKDIR /app
