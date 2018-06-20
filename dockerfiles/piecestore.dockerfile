# build
FROM golang:alpine AS build-env
RUN apk update
RUN apk upgrade
RUN apk add gcc musl-dev

ADD . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj/cmd/piecestore-farmer && go build -o piecestore-farmer

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/piecestore-farmer/piecestore-farmer /app/

RUN export PSID=$(./piecestore-farmer c --pieceStoreHost=${PUBLIC_IP} --pieceStorePort=${RPC_PORT} --kademliaPort=${KAD_PORT} --kademliaHost=${KAD_HOST} --kademliaListenPort=${KAD_LISTEN_PORT} --dir={$PS_DIR} | grep 'ID' | awk '{ print $2 }')
ENTRYPOINT ./piecestore-farmer s $PSID
