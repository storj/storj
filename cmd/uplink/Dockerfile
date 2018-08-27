# build
ARG GO_VERSION=1.10
FROM golang:${GO_VERSION}-alpine AS build-env
ARG GOOS=linux
ARG GOARCH=amd64
ARG LDFLAGS=
RUN apk add -U curl git musl-dev gcc
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj && dep ensure -vendor-only
ENV GOOS=${GOOS} \
    GOARCH=${GOARCH} \
    LDFLAGS=${LDFLAGS} \
    GOARM=6 \
    CGO_ENABLED=0
RUN cd /go/src/storj.io/storj/cmd/uplink \
 && go build -o uplink -ldflags "-s -w"


# final stage
FROM alpine
ENV API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7777


WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/uplink/uplink /app/
COPY cmd/uplink/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
