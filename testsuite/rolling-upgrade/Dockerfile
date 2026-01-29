# syntax=docker/dockerfile:1.4-labs
ARG GO_VERSION="1.25.6"

FROM golang:${GO_VERSION}

RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /workspace
COPY . .

ENTRYPOINT ["/workspace/testsuite/rolling-upgrade/start-sim.sh"]
