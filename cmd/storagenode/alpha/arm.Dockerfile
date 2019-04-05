# build
FROM golang:1.11-alpine as build-env

ADD . /go/src/storj.io/storj

WORKDIR /go/src/storj.io/storj/cmd/storagenode

RUN mkdir config

# final stage
FROM alpine

EXPOSE 28967

ENV STORJ_KADEMLIA_BOOTSTRAP_ADDR="bootstrap.storj.io:8888"
ENV STORJ_METRICS_APP_SUFFIX="-alpha"
ENV STORJ_METRICS_INTERVAL="30m"
ENV STORJ_SERVER_USE_PEER_CA_WHITELIST="true"
ENV COMMAND="run"

WORKDIR /app


COPY --from=build-env /go/src/storj.io/storj/storagenode_linux_arm /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/

ENTRYPOINT ./storagenode $COMMAND --config-dir="config"