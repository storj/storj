#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source $SCRIPTDIR/../../scripts/utils.sh

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    uplink rm "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile"
    uplink rm "sj://$BUCKET_WITH_ACCESS/another-testfile"
    uplink rm "sj://$BUCKET_WITHOUT_ACCESS/another-testfile"
    uplink rb "sj://$BUCKET_WITHOUT_ACCESS"
    uplink rb "sj://$BUCKET_WITH_ACCESS"
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT
trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

export UPLINK_CONFIG_DIR=$TMPDIR/uplink

# workaround for issues with automatic accepting monitoring question
# with first run we need to accept question y/n about monitoring
mkdir -p "$UPLINK_CONFIG_DIR"
touch "$UPLINK_CONFIG_DIR/config.ini"

uplink access import -f test-access "$GATEWAY_0_ACCESS" --use

BUCKET_WITHOUT_ACCESS=bucket1
BUCKET_WITH_ACCESS=bucket2

FOLDER_TO_SHARE_FILE=folder

SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file "2KiB"       "$SRC_DIR/another-testfile"  # create 2kb file of random bytes (inline)
random_bytes_file "5KiB"       "$SRC_DIR/testfile"          # create 5kb file of random bytes (remote)

uplink mb "sj://$BUCKET_WITHOUT_ACCESS/"
uplink mb "sj://$BUCKET_WITH_ACCESS/"

uplink cp "$SRC_DIR/testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/" --progress=false
uplink cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/" --progress=false
uplink cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITHOUT_ACCESS/" --progress=false

# Make access with readonly rights
SHARED_ACCESS=$(uplink share "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/" --readonly | grep Access | awk '{print $3}')
echo "Shared access: $SHARED_ACCESS"

uplink cp "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/" --access $SHARED_ACCESS --progress=false
require_error_exit_code $?

uplink cp "$SRC_DIR/testfile" "sj://$BUCKET_WITHOUT_ACCESS/" --access $SHARED_ACCESS --progress=false
require_error_exit_code $?

uplink cp "sj://$BUCKET_WITHOUT_ACCESS/another-testfile" "$SRC_DIR/" --access $SHARED_ACCESS --progress=false
require_error_exit_code $?

NUMBER_OF_BUCKETS=$(uplink ls --access $SHARED_ACCESS | grep -v '^CREATED' | wc -l)

# We share one bucket, so we expect to see only one bucket in the output of ls command
if [ $NUMBER_OF_BUCKETS -eq 1 ]; then
    echo "Number of shared buckets matches the expected result. PASSED"
else
    echo "List of buckets more than 1. FAILED"
    exit 1
fi

uplink cp "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile" "$DST_DIR" --access $SHARED_ACCESS --progress=false

compare_files "$SRC_DIR/testfile" "$DST_DIR/testfile"