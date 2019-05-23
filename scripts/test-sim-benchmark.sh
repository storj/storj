#!/bin/bash
set -ueo pipefail

# Purpose: This script executes uplink upload and download benchmark tests against storj-sim.
# Setup: Remove any existing uplink configs.
# Usage: from root of storj repo, run
#   $ storj-sim network test bash ./scripts/test-sim-benchmark.sh
# To run and filter out storj-sim logs, run:
#   $ storj-sim -x network test bash ./scripts/test-sim-benchmark.sh | grep -i "test.out"

SATELLITE_0_ADDR=${SATELLITE_0_ADDR:-127.0.0.1}

apiKey=$(storj-sim network env GATEWAY_0_API_KEY)
echo "apiKey:"
echo "$apiKey"

# run benchmark tests normally
echo
echo "Executing benchmark tests locally"
go test -bench . -benchmem ./cmd/uplink/cmd/

# run s3-benchmark with uplink
echo
echo "Executing s3-benchmark tests with uplink client..."
s3-benchmark --client=uplink --satellite="$SATELLITE_0_ADDR" --apikey="$apiKey"
