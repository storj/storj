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

random_bytes_file () {
    size=$1
    output=$2
    head -c $size </dev/urandom > $output
}

random_bytes_file "1MiB"    "$SRC_DIR/big-upload-testfile"

UPLINK_DEBUG_ADDR=""

export STORJ_ACCESS=$GATEWAY_0_ACCESS
export STORJ_DEBUG_ADDR=$UPLINK_DEBUG_ADDR

uplink mb "sj://$BUCKET/"
uplink cp "$SRC_DIR/big-upload-testfile" "sj://$BUCKET/" --progress=false