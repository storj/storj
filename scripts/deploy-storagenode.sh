#!/usr/bin/env bash

# Usage: TAG=6e8c4ed-v0.19.0-go1.12.9 scripts/deploy-storagenode.sh
set -euo pipefail

: "${TAG:?Must be set to the gitish version of the release without architecture}"

for v in alpha arm beta latest; do
	docker manifest create --amend storjlabs/storagenode:$v \
	storjlabs/storagenode:${TAG}-amd64 \
	storjlabs/storagenode:${TAG}-arm32v6 \
	storjlabs/storagenode:${TAG}-aarch64

	docker manifest annotate storjlabs/storagenode:$v \
	storjlabs/storagenode:${TAG}-amd64 --os linux --arch amd64

	docker manifest annotate storjlabs/storagenode:$v \
	storjlabs/storagenode:${TAG}-arm32v6 --os linux --arch arm --variant v6

	docker manifest annotate storjlabs/storagenode:$v \
	storjlabs/storagenode:${TAG}-aarch64 --os linux --arch arm64

	docker manifest push --purge storjlabs/storagenode:$v
done
