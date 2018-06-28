# build
FROM golang:alpine AS build-env
RUN apk update
RUN apk upgrade
RUN apk add gcc musl-dev

ADD . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj/cmd/piecestore-farmer && go build -o piecestore-farmer

# final stage
FROM alpine
ENV KAD_HOST= \
    KAD_LISTEN_PORT= \
    KAD_PORT= \
    PSID= \
    PS_DIR= \
    PUBLIC_IP= \
    RPC_PORT=

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/piecestore-farmer/piecestore-farmer /app/
COPY dockerfiles/piecestore-farmer-entrypoint /
ENTRYPOINT ["/piecestore-farmer-entrypoint"]
