#!/usr/bin/env bash

# Prints all packages of the module, with the slowest test packages first.
#
# go test starts packages in the order they are given. With the default
# ./... order, slow packages like storagenode/hashstore start near the end
# and are the last ones running. Keep the list ordered by test duration.

set -ueo pipefail

slow=(
	storj.io/storj/satellite/metabase
	storj.io/storj/satellite/metainfo
	storj.io/storj/satellite/payments/stripe
	storj.io/storj/satellite/repair
	storj.io/storj/satellite/overlay
	storj.io/storj/storagenode/hashstore
	storj.io/storj/satellite/satellitedb
	storj.io/storj/satellite/satellitedb/consoledb
	storj.io/storj/satellite/audit
	storj.io/storj/satellite
	storj.io/storj/satellite/payments/billing
	storj.io/storj/satellite/balancer
	storj.io/storj/satellite/reputation
)

all=$(go list ./...)

# Skip packages from the list that don't exist anymore.
for pkg in "${slow[@]}"; do
	grep -xF "$pkg" <<<"$all" || true
done
grep -vxF -f <(printf '%s\n' "${slow[@]}") <<<"$all"
