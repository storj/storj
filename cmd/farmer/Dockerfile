# build
FROM golang:1.10-alpine AS build-env
RUN apk add -U curl git musl-dev gcc \
 && apk add git
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj && dep ensure -vendor-only
RUN cd /go/src/storj.io/storj/cmd/farmer && go build -o farmer

# final stage
FROM alpine
ENV KAD_HOST=bootstrap.storj.io \
    KAD_LISTEN_PORT=7776 \
    KAD_PORT=8080 \
    PSID= \
    CONF_PATH= \
    PS_DIR=/home/ \
    PUBLIC_IP=127.0.0.1 \
    RPC_PORT=7777

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/farmer/farmer /app/
COPY cmd/farmer/entrypoint /farmer-entrypoint
ENTRYPOINT ["/farmer-entrypoint"]
