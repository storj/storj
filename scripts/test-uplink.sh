#!/usr/bin/env bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}

trap cleanup EXIT

BUCKET=bucket-123
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file () {
    size=$1
    output=$2
    dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

random_bytes_file 2x1024      "$SRC_DIR/small-upload-testfile" # create 2kb file of random bytes (inline)
random_bytes_file 5x1024x1024 "$SRC_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)

UPLINK_DEBUG_ADDR=""

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" mb "sj://$BUCKET/"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/big-upload-testfile" "sj://$BUCKET/"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "sj://$BUCKET/small-upload-testfile" "$DST_DIR"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "sj://$BUCKET/big-upload-testfile" "$DST_DIR"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rm "sj://$BUCKET/small-upload-testfile"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rm "sj://$BUCKET/big-upload-testfile"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" ls "sj://$BUCKET"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rb "sj://$BUCKET"

if cmp "$SRC_DIR/small-upload-testfile" "$DST_DIR/small-upload-testfile"
then
    echo "small upload testfile matches uploaded file"
else
    echo "small upload testfile does not match uploaded file"
    exit 1
fi

if cmp "$SRC_DIR/big-upload-testfile" "$DST_DIR/big-upload-testfile"
then
    echo "big upload testfile matches uploaded file"
else
    echo "big upload testfile does not match uploaded file"
    exit 1
fi

# check if all data files were removed
# FILES=$(find "$STORAGENODE_0_DIR/../" -type f -path "*/blob/*" ! -name "info.*")
# if [ -z "$FILES" ];
# then
#     echo "all data files removed from storage nodes"
# else
#     echo "not all data files removed from storage nodes:"
#     echo $FILES
#     exit 1
# fi
