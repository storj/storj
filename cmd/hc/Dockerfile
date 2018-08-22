ARG GO_VERSION=1.10
FROM golang:${GO_VERSION}-alpine AS build-env
RUN apk update && \
    apk upgrade && \
    apk add curl && \
    apk add git
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
COPY . /go/src/storj.io/storj
RUN cd /go/src/storj.io/storj && dep ensure -vendor-only
RUN cd /go/src/storj.io/storj/cmd/hc && go build -o satellite


# final stage
FROM alpine
ENV REDIS_ADDRESS= \
    REDIS_PASSWORD= \
    REDIS_DB=0 \
    OVERLAY_PORT=:7070 \
    HTTP_PORT=:8080
WORKDIR /app
COPY --from=build-env /go/src/storj.io/storj/cmd/hc/satellite /app/
COPY cmd/hc/entrypoint /entrypoint
EXPOSE 8081/udp \
    8080 \
    7070

ENTRYPOINT ["/entrypoint"]
