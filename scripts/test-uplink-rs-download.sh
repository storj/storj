#!/usr/bin/env bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}

trap cleanup EXIT

BUCKET=bucket-for-rs-change
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst
UPLINK_DIR=$TMPDIR/uplink

mkdir -p "$SRC_DIR" "$DST_DIR"

UPLINK_DEBUG_ADDR=""

export STORJ_ACCESS=$GATEWAY_0_ACCESS
export STORJ_DEBUG_ADDR=$UPLINK_DEBUG_ADDR

uplink cp  "sj://$BUCKET/big-upload-testfile" "$DST_DIR" --progress=false