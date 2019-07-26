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
RELEASE_DIR=${RELEASE_DIR:-"releasefir/dst"}
WORKDIR=${WORKDIR:-"dst"}

mkdir -p "$SRC_DIR" "$RELEASE_DIR" "$WORKDIR"

random_bytes_file () {
    size=$1
    output=$2
    dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

random_bytes_file 2x1024      "$SRC_DIR/small-upload-testfile" # create 2kb file of random bytes (inline)
random_bytes_file 5x1024x1024 "$SRC_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)

# upload files on the most recent release tag
if [ "$1" == "upload" ]; then
    uplink --config-dir "$GATEWAY_0_DIR" mb "sj://$BUCKET/"

    uplink --config-dir "$GATEWAY_0_DIR" cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/"
    uplink --config-dir "$GATEWAY_0_DIR" cp "$SRC_DIR/big-upload-testfile" "sj://$BUCKET/"

    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/small-upload-testfile" "$RELEASE_DIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/big-upload-testfile" "$RELEASE_DIR"

    if cmp "$SRC_DIR/small-upload-testfile" "$RELEASE_DIR/small-upload-testfile"
    then
        echo "small upload testfile matches uploaded file"
    else
        echo "small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$SRC_DIR/big-upload-testfile" "$RELEASE_DST_DIR/big-upload-testfile"
    then
        echo "big upload testfile matches uploaded file"
    else
        echo "big upload testfile does not match uploaded file"
        exit 1
    fi
fi

# return to the workdir that contains the current branch under test
# and download the files to confirm backward compatibility
if [ "$1" == "download" ]; then
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/small-upload-testfile" "$WORKDIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/big-upload-testfile" "$WORKDIR"

    if cmp "$SRC_DIR/small-upload-testfile" "$WORKDIR/small-upload-testfile"
    then
        echo "small upload testfile matches uploaded file"
    else
        echo "small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$SRC_DIR/big-upload-testfile" "$WORKDIR/big-upload-testfile"
    then
        echo "big upload testfile matches uploaded file"
    else
        echo "big upload testfile does not match uploaded file"
        exit 1
    fi
fi

# cleanup
if [ "$1" == "cleanup" ]; then
    uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/small-upload-testfile"
    uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/big-upload-testfile"
    uplink --config-dir "$GATEWAY_0_DIR" rb "sj://$BUCKET"
fi