#!/usr/bin/env bash

# Usage: VERSION=0.3.8 scripts/deploy-watchtower.sh
set -euo pipefail

: "${VERSION:?Must be set to the release version}"

docker manifest create storjlabs/watchtower:latest \
storjlabs/watchtower:i386-${VERSION} \
storjlabs/watchtower:amd64-${VERSION} \
storjlabs/watchtower:arm64v8-${VERSION} \
storjlabs/watchtower:armhf-${VERSION}

docker manifest annotate storjlabs/watchtower:latest \
storjlabs/watchtower:i386-${VERSION} --os linux --arch 386

docker manifest annotate storjlabs/watchtower:latest \
storjlabs/watchtower:amd64-${VERSION} --os linux --arch amd64

docker manifest annotate storjlabs/watchtower:latest \
storjlabs/watchtower:arm64v8-${VERSION} --os linux --arch arm64 --variant v8

docker manifest annotate storjlabs/watchtower:latest \
storjlabs/watchtower:armhf-${VERSION} --os linux --arch arm

docker manifest push --purge storjlabs/watchtower:latest
