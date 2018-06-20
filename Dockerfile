# build
FROM golang:1.10 AS build-env
ADD . /go/src/storj.io/storj
RUN go version && go get -u -v golang.org/x/vgo
RUN vgo install ./...
RUN cd /go/src/storj.io/storj/cmd/overlay && go build -o overlay


# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/overlay/overlay /app/

ENTRYPOINT ./overlay -redisAddress=${REDIS_ADDRESS} -redisPassword=${REDIS_PASSWORD} -db=${REDIS_DB} -srvPort=${OVERLAY_PORT} -httpPort=${HTTP_PORT}