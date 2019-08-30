#!/bin/bash
set -ueo pipefail

docker build -f .Dockerfile.ci \
    -t gcr.io/storj-jessica/sa-k8s-hard-way/ci:$CIRCLE_SHA1 .

docker push gcr.io/storj-jessica/sa-k8s-hard-way/ci:$CIRCLE_SHA1

# TODO: if env production then do all the tardigrade theme stuff
