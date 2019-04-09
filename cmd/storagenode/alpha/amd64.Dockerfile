# build
FROM golang:1.11-alpine as build-env

ENV CGO_ENABLED=1

ADD . /go/src/storj.io/storj
WORKDIR /go/src/storj.io/storj/cmd/storagenode

# dependencies + binary
RUN apk add git gcc musl-dev
#RUN unset GOPATH && go mod vendor
RUN go build -a -installsuffix cgo -o storagenode .
RUN mkdir config identity

# final stage
FROM alpine

EXPOSE 28967

ENV STORJ_KADEMLIA_BOOTSTRAP_ADDR="bootstrap.storj.io:8888"
ENV STORJ_METRICS_APP_SUFFIX="-alpha"
ENV STORJ_METRICS_INTERVAL="30m"
ENV STORJ_SERVER_USE_PEER_CA_WHITELIST="true"

ENV ADDRESS=""
ENV EMAIL=""
ENV WALLET=""
ENV BANDWIDTH="2.0TB"
ENV STORAGE="2.0TB"


WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/config /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/identity /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/alpha/entrypoint.sh /app/
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/alpha/dashboard.sh /app/

RUN ls -l /app

ENTRYPOINT ["./entrypoint.sh"]
#ENTRYPOINT ./storagenode run --config-dir="/app/config" --identity-dir="/app/identity" --kademlia.external-address=${ADDRESS} --kademlia.operator.email=${EMAIL} --kademlia.operator.wallet=${WALLET}