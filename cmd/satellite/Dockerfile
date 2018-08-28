ARG GO_VERSION=1.11
FROM golang:${GO_VERSION}-alpine AS build-env
ARG GOOS=linux
ARG GOARCH=amd64
ARG LDFLAGS=
RUN apk add -U curl git musl-dev gcc
COPY . /go/src/storj.io/storj
ENV GOOS=${GOOS} \
    GOARCH=${GOARCH} \
    LDFLAGS=${LDFLAGS} \
    GOARM=6 \
    CGO_ENABLED=0 \
    GO111MODULE=on
RUN cd /go/src/storj.io/storj/cmd/satellite \
 && go build -o satellite -ldflags "-s -w $LDFLAGS"


# final stage
FROM alpine
ENV REDIS_ADDR=redis:6379 \
    REDIS_PASSWORD= \
    REDIS_DB=0 \
    IDENTITY_ADDR=:7777 \
    HTTP_PORT=:8080 \
    BOOTSTRAP_ADDR=bootstrap.storj.io:8080
WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/satellite/satellite /app/
COPY cmd/satellite/entrypoint /entrypoint
EXPOSE 8081/udp \
    8080 \
    7070

ENTRYPOINT ["/entrypoint"]
