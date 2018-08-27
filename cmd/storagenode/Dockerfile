# build
ARG GO_VERSION=1.10
FROM golang:${GO_VERSION}-alpine AS build-env
RUN apk add -U curl git musl-dev gcc
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj && dep ensure -vendor-only
RUN cd /go/src/storj.io/storj/cmd/storagenode && go build -o storagenode

# final stage
FROM alpine
ENV CONF_PATH= \
    SATELLITE_ADDR=
EXPOSE 7777

WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/storagenode/storagenode /app/
COPY cmd/storagenode/entrypoint /entrypoint
ENTRYPOINT ["/entrypoint"]
