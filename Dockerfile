# env
ARG VERSION
# build
FROM golang:${VERSION} AS build-env
COPY . /go/src/storj.io/storj
RUN go get -u -v golang.org/x/vgo
RUN cd /go/src/storj.io/storj && vgo install ./...
RUN cd /go/src/storj.io/storj/cmd/overlay && vgo build -o overlay


# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/overlay/overlay /app/

ENTRYPOINT ./overlay -redisAddress=${REDIS_ADDRESS} -redisPassword=${REDIS_PASSWORD} -db=${REDIS_DB} -srvPort=${OVERLAY_PORT} -httpPort=${HTTP_PORT}