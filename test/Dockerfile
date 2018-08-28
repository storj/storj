FROM golang:1.11-alpine

RUN apk -U add make git bash gcc musl-dev

COPY . /go/src/storj.io/storj

WORKDIR /go/src/storj.io/storj

RUN make build-dev-deps
