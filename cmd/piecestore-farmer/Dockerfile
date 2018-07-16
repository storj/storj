# build
FROM golang:1.10-alpine AS build-env
RUN apk add -U curl git musl-dev gcc \
 && apk add git
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj && dep ensure -vendor-only
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
COPY cmd/piecestore-farmer/entrypoint /piecestore-farmer-entrypoint
ENTRYPOINT ["/piecestore-farmer-entrypoint"]
