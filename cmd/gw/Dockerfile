# build
ARG GO_VERSION=1.10
FROM golang:${GO_VERSION}-alpine AS build-env
RUN apk add -U curl git musl-dev gcc
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
WORKDIR /go/src/storj.io/storj
COPY . .
RUN dep ensure -vendor-only
RUN go build -o uplink cmd/gw/*.go

# final stage
FROM alpine
ENV API_KEY= \
    SATELLITE_ADDR=
EXPOSE 7777


WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/uplink /app/
COPY cmd/gw/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
