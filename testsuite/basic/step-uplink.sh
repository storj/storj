#!/usr/bin/env bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source $SCRIPTDIR/../../scripts/utils.sh

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT
trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

BUCKET=bucket-123
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

export UPLINK_CONFIG_DIR=$TMPDIR/uplink

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file "2KiB"    "$SRC_DIR/small-upload-testfile"          # create 2KiB file of random bytes (inline)
random_bytes_file "5MiB"    "$SRC_DIR/big-upload-testfile"            # create 5MiB file of random bytes (remote)
# this is special case where we need to test at least one remote segment and inline segment of exact size 0
random_bytes_file "12MiB"   "$SRC_DIR/multisegment-upload-testfile"   # create 12MiB file of random bytes (1 remote segments + inline)
random_bytes_file "13MiB"   "$SRC_DIR/diff-size-segments"             # create 13MiB file of random bytes (2 remote segments)
random_bytes_file "15MiB"   "$SRC_DIR/put-file"

# workaround for issues with automatic accepting monitoring question
# with first run we need to accept question y/n about monitoring
mkdir -p "$UPLINK_CONFIG_DIR"
touch "$UPLINK_CONFIG_DIR/config.ini"

uplink access import -f test-access "$GATEWAY_0_ACCESS" --use

uplink mb "sj://$BUCKET/"

uplink cp "$SRC_DIR/small-upload-testfile"        "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/big-upload-testfile"          "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/multisegment-upload-testfile" "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/diff-size-segments"           "sj://$BUCKET/" --progress=false
cat "$SRC_DIR/put-file" | uplink cp -             "sj://$BUCKET/put-file" --progress=false

# check overwriting of existing object
uplink cp "$SRC_DIR/big-upload-testfile"          "sj://$BUCKET/" --progress=false

# test parallelism to upload single object
# TODO change hardcoded part size from 64MiB to 6MiB
uplink cp "$SRC_DIR/diff-size-segments"           "sj://$BUCKET/diff-size-segments_upl_p2" --progress=false --parallelism 2

# check named access
uplink access import -f named-access "$GATEWAY_0_ACCESS"
FILES=$(uplink ls "sj://$BUCKET" --access named-access | tee "$TMPDIR/list" | wc -l)
EXPECTED_FILES="7" # 6 objects + one line more for headers
if [ "$FILES" == "$EXPECTED_FILES" ]
then
    echo "listing returns $FILES files"
else
    echo "listing returns $FILES files but want $EXPECTED_FILES"
    cat "$TMPDIR/list"
    exit 1
fi

SIZE_CHECK=$(cat "$TMPDIR/list" | awk '{if($4 == "0") print "invalid size";}')
if [ "$SIZE_CHECK" != "" ]
then
    echo "listing returns invalid size for one of the objects:"
    cat "$TMPDIR/list"
    exit 1
fi

uplink ls "sj://$BUCKET/non-existing-prefix"

uplink cp "sj://$BUCKET/small-upload-testfile"        "$DST_DIR" --progress=false
uplink cp "sj://$BUCKET/big-upload-testfile"          "$DST_DIR" --progress=false
uplink cp "sj://$BUCKET/multisegment-upload-testfile" "$DST_DIR" --progress=false
uplink cp "sj://$BUCKET/diff-size-segments"           "$DST_DIR" --progress=false
uplink cp "sj://$BUCKET/put-file"                     "$DST_DIR" --progress=false

# test parallelism of single object
uplink cp "sj://$BUCKET/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile_p2" --parallelism 2 --progress=false
uplink cp "sj://$BUCKET/diff-size-segments"           "$DST_DIR/diff-size-segments_p2"           --parallelism 2 --progress=false
uplink cp "sj://$BUCKET/diff-size-segments_upl_p2"    "$DST_DIR/diff-size-segments_upl_p2"       --parallelism 2 --progress=false

uplink cp "sj://$BUCKET/put-file" - --parallelism 3 --parallelism-chunk-size 64MiB --progress=false >> "$DST_DIR/put-file_p2"

uplink ls "sj://$BUCKET/small-upload-testfile" | grep "small-upload-testfile"

# test ranged download of object
uplink cp "sj://$BUCKET/small-upload-testfile" "$DST_DIR/file-from-cp-range" --progress=false --range bytes=0-5
EXPECTED_FILE_SIZE="6"
ACTUAL_FILE_SIZE=$(get_file_size "$DST_DIR/file-from-cp-range")
if [ "$EXPECTED_FILE_SIZE" != "$ACTUAL_FILE_SIZE" ]
then
    echo "expected downloaded file size to be equal to $EXPECTED_FILE_SIZE, got $ACTUAL_FILE_SIZE"
    exit 1
fi

# test server-side move operation
uplink mv "sj://$BUCKET/big-upload-testfile"       "sj://$BUCKET/moved-big-upload-testfile"
uplink ls "sj://$BUCKET/moved-big-upload-testfile" | grep "moved-big-upload-testfile"
uplink mv "sj://$BUCKET/moved-big-upload-testfile" "sj://$BUCKET/big-upload-testfile"

# test server-side copy operation
uplink cp "sj://$BUCKET/big-upload-testfile"        "sj://$BUCKET/copied-big-upload-testfile"
uplink ls "sj://$BUCKET/copied-big-upload-testfile" | grep "copied-big-upload-testfile"
uplink ls "sj://$BUCKET/big-upload-testfile"        | grep "big-upload-testfile"
uplink cp "sj://$BUCKET/copied-big-upload-testfile" "$DST_DIR/copied-big-upload-testfile"
compare_files "$SRC_DIR/big-upload-testfile"        "$DST_DIR/copied-big-upload-testfile"

# move prefix
uplink mv "sj://$BUCKET/" "sj://$BUCKET/my-prefix/" --recursive
FILES=$(uplink ls "sj://$BUCKET/my-prefix/" | tee "$TMPDIR/list" | wc -l)
EXPECTED_FILES="8" # 7 objects + one line more for headers
if [ "$FILES" == "$EXPECTED_FILES" ]
then
    echo "listing after move returns $FILES files"
else
    echo "listing after move returns $FILES files but want $EXPECTED_FILES"
    cat "$TMPDIR/list"
    exit 1
fi
uplink mv "sj://$BUCKET/my-prefix/" "sj://$BUCKET/" --recursive

uplink rm "sj://$BUCKET/small-upload-testfile"
uplink rm "sj://$BUCKET/big-upload-testfile"
uplink rm "sj://$BUCKET/multisegment-upload-testfile"
uplink rm "sj://$BUCKET/diff-size-segments"
uplink rm "sj://$BUCKET/put-file"
uplink rm "sj://$BUCKET/diff-size-segments_upl_p2"
uplink rm "sj://$BUCKET/copied-big-upload-testfile"

uplink ls "sj://$BUCKET"
uplink ls -x "sj://$BUCKET"

uplink rb "sj://$BUCKET"

compare_files "$SRC_DIR/small-upload-testfile"        "$DST_DIR/small-upload-testfile"
compare_files "$SRC_DIR/big-upload-testfile"          "$DST_DIR/big-upload-testfile"
compare_files "$SRC_DIR/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments"
compare_files "$SRC_DIR/put-file"                     "$DST_DIR/put-file"

# test parallelism of single object
compare_files "$SRC_DIR/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile_p2"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments_p2"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments_upl_p2"
compare_files "$SRC_DIR/put-file"                     "$DST_DIR/put-file_p2"

# test deleting non empty bucket with --force flag
uplink mb "sj://$BUCKET/"

for i in $(seq -w 1 16); do
  uplink cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/small-file-$i" --progress=false
done

uplink rb "sj://$BUCKET" --force

if [ "$(uplink ls | wc -l)" != "0" ]; then
  echo "an integration test did not clean up after itself entirely"
  exit 1
fi
