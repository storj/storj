#!/bin/bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
# cleanup() {
#     rm -rf "$TMPDIR"
# }
# trap cleanup EXIT

go install storj.io/storj/cmd/uplink
uplink setup --config-dir $TMPDIR
uplink --config-dir="$TMPDIR" mb sj://testbucket/

