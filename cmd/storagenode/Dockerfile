
# build
FROM golang:1.11-alpine as build-env
ADD . /go/src/storj.io/storj
WORKDIR /go/src/storj.io/storj/cmd/storagenode

ENV COMMAND=run

ENV STORJ_KADEMLIA_BOOTSTRAP_ADDR="bootstrap.storj.io:21627"
ENV STORJ_METRICS_APP_SUFFIX="-alpha"
ENV STORJ_METRICS_INTERVAL="30m"
ENV STORJ_SERVER_EXTENSIONS_WHITELIST_SIGNED_LEAF=true
ENV STORJ_SERVER_PEER_CA_WHITELIST_PATH="?"
ENV STORJ_SIGNER_ADDRESS="?"
ENV STORJ_SIGNER_AUTH_TOKEN="?"


# dependencies + binary
# RUN unset GOPATH && go mod vendor
RUN apk add --no-cache git gcc musl-dev && unset GOPATH && go mod vendor && CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o storagenode . && mkdir config

# final stage
FROM alpine

EXPOSE 28967
EXPOSE 7777

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/

ENTRYPOINT ./storagenode $COMMAND --config-dir="config"