# build
FROM golang:1.11-alpine as build-env
ADD . /go/src/storj.io/storj
WORKDIR /go/src/storj.io/storj/cmd/storagenode

# dependencies + binary
# RUN unset GOPATH && go mod vendor
RUN apk add --no-cache git gcc musl-dev && unset GOPATH && go mod vendor && CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o storagenode . && mkdir config
# RUN apk add --no-cache gcc musl-dev && unset GOPATH && CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o storagenode . && mkdir config

# final stage
FROM alpine

EXPOSE 28967
EXPOSE 7777

ENV STORJ_KADEMLIA_BOOTSTRAP_ADDR="bootstrap.storj.io:8888"
ENV STORJ_METRICS_APP_SUFFIX="-alpha"
ENV STORJ_METRICS_INTERVAL="30m"
ENV STORJ_SERVER_USE_PEER_CA_WHITELIST="true"
ENV COMMAND="run"

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/

ENTRYPOINT ./storagenode $COMMAND --config-dir="config" 
