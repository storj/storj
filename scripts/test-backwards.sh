#!/usr/bin/env bash
set -ueo pipefail

BUCKET=bucket-123
TEST_FILES_DIR=testfiles
BRANCH_DST_DIR=branch
RELEASE_DST_DIR=release
# BRANCH_DST_DIR=${BRANCH_DIR:-$TMPDIR/branch}
# RELEASE_DST_DIR=${RELEASE_DIR:-$TMPDIR/release}

setup(){
    mkdir -p "$TEST_FILES_DIR" "$BRANCH_DST_DIR" "$RELEASE_DST_DIR"
    random_bytes_file () {
        size=$1
        output=$2
        dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
    }
    random_bytes_file 2x1024      "$TEST_FILES_DIR/small-upload-testfile" # create 2kb file of random bytes (inline)
    random_bytes_file 5x1024x1024 "$TEST_FILES_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)
    echo "setup test successfully"
}

cleanup(){
    rm -rf "$BRANCH_DST_DIR"
    rm -rf "$RELEASE_DST_DIR"
    rm -rf "$TEST_FILES_DIR"
    echo "cleaned up test successfully"
}

if [[ "$1" == "upload" ]]; then
    setup

    uplink --config-dir "$GATEWAY_0_DIR" mb "sj://$BUCKET/"

    uplink --config-dir "$GATEWAY_0_DIR" cp "$TEST_FILES_DIR/small-upload-testfile" "sj://$BUCKET/"
    uplink --config-dir "$GATEWAY_0_DIR" cp "$TEST_FILES_DIR/big-upload-testfile" "sj://$BUCKET/"

    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/small-upload-testfile" "$RELEASE_DST_DIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/big-upload-testfile" "$RELEASE_DST_DIR"

    if cmp "$TEST_FILES_DIR/small-upload-testfile" "$RELEASE_DST_DIR/small-upload-testfile"
    then
        echo "upload test on release tag: small upload testfile matches uploaded file"
    else
        echo "upload test on release tag: small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$TEST_FILES_DIR/big-upload-testfile" "$RELEASE_DST_DIR/big-upload-testfile"
    then
        echo "upload test on release tag: big upload testfile matches uploaded file"
    else
        echo "upload test on release tag: big upload testfile does not match uploaded file"
        exit 1
    fi
fi

if [[ "$1" == "download" ]]; then
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/small-upload-testfile" "$BRANCH_DST_DIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp "sj://$BUCKET/big-upload-testfile" "$BRANCH_DST_DIR"

    if cmp "$TEST_FILES_DIR/small-upload-testfile" "$BRANCH_DST_DIR/small-upload-testfile"
    then
        echo "download test on current branch: small upload testfile matches uploaded file"
    else
        echo "download test on current branch: small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$TEST_FILES_DIR/big-upload-testfile" "$BRANCH_DST_DIR/big-upload-testfile"
    then
        echo "download test on current branch: big upload testfile matches uploaded file"
    else
        echo "download test on current branch: big upload testfile does not match uploaded file"
        exit 1
    fi

    uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/small-upload-testfile"
    uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/big-upload-testfile"
    uplink --config-dir "$GATEWAY_0_DIR" rb "sj://$BUCKET"
    cleanup
fi
