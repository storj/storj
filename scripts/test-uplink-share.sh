#!/usr/bin/env bash

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    uplink --access "$GATEWAY_0_ACCESS" rm "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile"
    uplink --access "$GATEWAY_0_ACCESS" rm "sj://$BUCKET_WITH_ACCESS/another-testfile"
    uplink --access "$GATEWAY_0_ACCESS" rm "sj://$BUCKET_WITHOUT_ACCESS/another-testfile"
    uplink --access "$GATEWAY_0_ACCESS" rb "sj://$BUCKET_WITHOUT_ACCESS"
    uplink --access "$GATEWAY_0_ACCESS" rb "sj://$BUCKET_WITH_ACCESS"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

require_error_exit_code(){
    if [ $1 -eq 0 ]; then
        echo "Result of copying does not match expectations. Test FAILED"
        exit 1
    else
        echo "Copy file without permission: PASSED"    # Expect unsuccessful exit code
    fi
}

random_bytes_file () {
    size=$1
    output=$2
    head -c $size </dev/urandom > $output
}

BUCKET_WITHOUT_ACCESS=bucket1
BUCKET_WITH_ACCESS=bucket2

FOLDER_TO_SHARE_FILE=folder

SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file "2KiB"       "$SRC_DIR/another-testfile"  # create 2kb file of random bytes (inline)
random_bytes_file "5KiB"       "$SRC_DIR/testfile"          # create 5kb file of random bytes (remote)

uplink --access "$GATEWAY_0_ACCESS" mb "sj://$BUCKET_WITHOUT_ACCESS/"
uplink --access "$GATEWAY_0_ACCESS" mb "sj://$BUCKET_WITH_ACCESS/"

uplink --access "$GATEWAY_0_ACCESS" cp "$SRC_DIR/testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/"
uplink --access "$GATEWAY_0_ACCESS" cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/"
uplink --access "$GATEWAY_0_ACCESS" cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITHOUT_ACCESS/"

# Make access with readonly rights
SHARED_ACCESS=$(uplink --access "$GATEWAY_0_ACCESS" share --allowed-path-prefix sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/ --readonly | grep Access | cut -d: -f2)

uplink cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/" --access $SHARED_ACCESS
require_error_exit_code $?

uplink cp "$SRC_DIR/testfile" "sj://$BUCKET_WITHOUT_ACCESS/" --access $SHARED_ACCESS
require_error_exit_code $?

uplink cp "sj://$BUCKET_WITHOUT_ACCESS/another-testfile" "$SRC_DIR/" --access $SHARED_ACCESS
require_error_exit_code $?

NUMBER_OF_BUCKETS=$(uplink ls --access $SHARED_ACCESS | wc -l)

# We share one bucket, so we expect to see only one bucket in the output of ls command
if [ $NUMBER_OF_BUCKETS -eq 1 ]; then
    echo "Number of shared buckets matches the expected result. PASSED"
else
    echo "List of buckets more than 1. FAILED"
    exit 1
fi

uplink cp "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile" "$DST_DIR" --access $SHARED_ACCESS

if cmp "$SRC_DIR/testfile" "$DST_DIR/testfile"; then
    echo "Testfile matches uploaded file: PASSED"
else
    echo "Download test: FAILED"
    exit 1
fi
