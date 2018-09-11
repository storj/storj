FROM golang:1.10

RUN apt-get update \
 && apt-get install -y build-essential awscli

RUN cd / \
 && rm -rf /go \
 && git clone --recursive https://github.com/storj/storj-vendor.git /go \
 && cd /go \
 && ./setup.sh

COPY . /go/src/storj.io/storj

WORKDIR /go/src/storj.io/storj

RUN make build-dev-deps
