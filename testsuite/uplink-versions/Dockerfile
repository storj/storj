# syntax=docker/dockerfile:1.4-labs
ARG GO_VERSION="1.25.6"

FROM golang:${GO_VERSION}

RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /workspace
RUN go install golang.org/dl/go1.16.15@latest && go1.16.15 download

COPY . .

ENTRYPOINT ["/workspace/testsuite/uplink-versions/start-sim.sh"]
