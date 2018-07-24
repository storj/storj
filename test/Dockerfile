FROM golang:1.10-alpine

RUN apk -U add make git bash gcc musl-dev

RUN cd / \
 && rm -rf /go \
 && git clone --recursive https://github.com/storj/storj-vendor.git /go \
 && cd /go \
 && ./setup.sh

COPY . /go/src/storj.io/storj

WORKDIR /go/src/storj.io/storj

RUN make build-dev-deps
