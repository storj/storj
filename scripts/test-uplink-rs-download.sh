#!/usr/bin/env bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}

trap cleanup EXIT

# workaround for issues with automatic accepting monitoring question
# with first run we need to accept question y/n about monitoring
export UPLINK_CONFIG_DIR=$TMPDIR/uplink
mkdir -p "$UPLINK_CONFIG_DIR"
touch "$UPLINK_CONFIG_DIR/config.ini"

uplink access import -f test-access "$GATEWAY_0_ACCESS" --use

BUCKET=bucket-for-rs-change
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

uplink cp  "sj://$BUCKET/big-upload-testfile" "$DST_DIR" --progress=false