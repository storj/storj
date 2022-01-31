#!/usr/bin/env bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source $SCRIPTDIR/utils.sh

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
UPLINK_DIR=$TMPDIR/uplink

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file "2KiB"    "$SRC_DIR/small-upload-testfile"          # create 2KiB file of random bytes (inline)
random_bytes_file "5MiB"    "$SRC_DIR/big-upload-testfile"            # create 5MiB file of random bytes (remote)
# this is special case where we need to test at least one remote segment and inline segment of exact size 0
random_bytes_file "12MiB"   "$SRC_DIR/multisegment-upload-testfile"   # create 12MiB file of random bytes (1 remote segments + inline)
random_bytes_file "13MiB"   "$SRC_DIR/diff-size-segments"             # create 13MiB file of random bytes (2 remote segments)

random_bytes_file "100KiB"  "$SRC_DIR/put-file"                       # create 100KiB file of random bytes (remote)

UPLINK_DEBUG_ADDR=""

export STORJ_ACCESS=$GATEWAY_0_ACCESS
export STORJ_DEBUG_ADDR=$UPLINK_DEBUG_ADDR

uplink mb "sj://$BUCKET/"

uplink cp "$SRC_DIR/small-upload-testfile"        "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/big-upload-testfile"          "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/multisegment-upload-testfile" "sj://$BUCKET/" --progress=false
uplink cp "$SRC_DIR/diff-size-segments"           "sj://$BUCKET/" --progress=false

# test parallelism to upload single object
# TODO change hardcoded part size from 64MiB to 6MiB
uplink cp "$SRC_DIR/diff-size-segments"           "sj://$BUCKET/diff-size-segments_upl_p2" --progress=false --parallelism 2

cat "$SRC_DIR/put-file" | uplink put "sj://$BUCKET/put-file"

uplink --config-dir "$UPLINK_DIR" import named-access $STORJ_ACCESS
FILES=$(STORJ_ACCESS= uplink --config-dir "$UPLINK_DIR" --access named-access ls "sj://$BUCKET" | tee $TMPDIR/list | wc -l)
EXPECTED_FILES="6"
if [ "$FILES" == $EXPECTED_FILES ]
then
    echo "listing returns $FILES files"
else
    echo "listing returns $FILES files but want $EXPECTED_FILES"
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

uplink cp  "sj://$BUCKET/small-upload-testfile"        "$DST_DIR" --progress=false
uplink cp  "sj://$BUCKET/big-upload-testfile"          "$DST_DIR" --progress=false
uplink cp  "sj://$BUCKET/multisegment-upload-testfile" "$DST_DIR" --progress=false
uplink cp  "sj://$BUCKET/diff-size-segments"           "$DST_DIR" --progress=false
uplink cp  "sj://$BUCKET/put-file"                     "$DST_DIR" --progress=false
uplink cat "sj://$BUCKET/put-file" >>                  "$DST_DIR/put-file-from-cat"

# test parallelism of single object
uplink cp "sj://$BUCKET/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile_p2" --parallelism 2 --progress=false
uplink cp "sj://$BUCKET/diff-size-segments"           "$DST_DIR/diff-size-segments_p2"           --parallelism 2 --progress=false
uplink cp "sj://$BUCKET/diff-size-segments_upl_p2"    "$DST_DIR/diff-size-segments_upl_p2"       --parallelism 2 --progress=false

uplink ls "sj://$BUCKET/small-upload-testfile" | grep "small-upload-testfile"

# test ranged download of object
uplink cp "sj://$BUCKET/put-file" "$DST_DIR/put-file-from-cp-range" --range bytes=0-5 --progress=false
EXPECTED_FILE_SIZE="6"
ACTUAL_FILE_SIZE=$(get_file_size "$DST_DIR/put-file-from-cp-range")
if [ "$EXPECTED_FILE_SIZE" != "$ACTUAL_FILE_SIZE" ]
then
    echo "expected downloaded file size to be equal to $EXPECTED_FILE_SIZE, got $ACTUAL_FILE_SIZE"
    exit 1
fi

# test ranged download with multiple byte range
set +e
EXPECTED_ERROR="retrieval of multiple byte ranges of data not supported: 2 provided"
ERROR=$(uplink cp "sj://$BUCKET/put-file" "$DST_DIR/put-file-from-cp-range" --range bytes=0-5,6-10)
if [ $ERROR != $EXPECTED_ERROR ]
then
    echo EXPECTED_ERROR
    exit 1
fi
set -e

# test server-side move operation
uplink mv "sj://$BUCKET/big-upload-testfile"       "sj://$BUCKET/moved-big-upload-testfile"
uplink ls "sj://$BUCKET/moved-big-upload-testfile" | grep "moved-big-upload-testfile"
uplink mv "sj://$BUCKET/moved-big-upload-testfile" "sj://$BUCKET/big-upload-testfile"

# test server-side move operation between different prefixes.

# destination and source should both be prefixes.
set +e
EXPECTED_ERROR="both source and destination should be a prefixes"
ERROR=$(uplink mv "sj://$BUCKET/" "sj://$BUCKET/new-prefix/file")
if [ $ERROR != $EXPECTED_ERROR ]
then
    echo EXPECTED_ERROR
    exit 1
fi
set -e

# checking if all files are moved from bucket to bucket/prefix.
EXPECTED_FILES=$(uplink ls "sj://$BUCKET/" | wc -l)
uplink mv "sj://$BUCKET/" "sj://$BUCKET/new-prefix/"
FILES=$(uplink ls "sj://$BUCKET/new-prefix/" | wc -l)
if [ "$FILES" == $EXPECTED_FILES ]
then
    echo "listing returns $FILES files as expected"
else
    echo "listing returns $FILES files but want $EXPECTED_FILES"
    exit 1
fi

# moving files back.
uplink mv "sj://$BUCKET/new-prefix/" "sj://$BUCKET/"
uplink ls "sj://$BUCKET/"

uplink rm "sj://$BUCKET/small-upload-testfile"
uplink rm "sj://$BUCKET/big-upload-testfile"
uplink rm "sj://$BUCKET/multisegment-upload-testfile"
uplink rm "sj://$BUCKET/diff-size-segments"
uplink rm "sj://$BUCKET/diff-size-segments_upl_p2"
uplink rm "sj://$BUCKET/put-file"
uplink ls "sj://$BUCKET"

uplink rb "sj://$BUCKET"

compare_files "$SRC_DIR/small-upload-testfile"        "$DST_DIR/small-upload-testfile"
compare_files "$SRC_DIR/big-upload-testfile"          "$DST_DIR/big-upload-testfile"
compare_files "$SRC_DIR/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments"
compare_files "$SRC_DIR/put-file"                     "$DST_DIR/put-file"
compare_files "$SRC_DIR/put-file"                     "$DST_DIR/put-file-from-cat"

# test parallelism of single object
compare_files "$SRC_DIR/multisegment-upload-testfile" "$DST_DIR/multisegment-upload-testfile_p2"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments_p2"
compare_files "$SRC_DIR/diff-size-segments"           "$DST_DIR/diff-size-segments_upl_p2"

# test deleting non empty bucket with --force flag
uplink mb "sj://$BUCKET/"

for i in $(seq -w 1 16); do
  uplink cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/small-file-$i" --progress=false
done

uplink rb "sj://$BUCKET" --force

if [ "$(uplink ls | grep "No buckets" | wc -l)" = "0" ]; then
  echo "an integration test did not clean up after itself entirely"
  exit 1
fi
