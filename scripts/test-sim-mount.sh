#!/bin/bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}

trap cleanup EXIT

BUCKET=bucket-storj-mount
SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst
MOUNT_DIR=$TMPDIR/dst/mount

mkdir -v -p "$SRC_DIR" "$DST_DIR" "$MOUNT_DIR"

random_bytes_file () {
    size=$1
    output=$2
    dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

random_bytes_file 5x1024x1024 "$SRC_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)

uplink --config-dir "$GATEWAY_0_DIR" mb "sj://$BUCKET/" 

# run in background
storj-mount run --config-dir "$GATEWAY_0_DIR"  --log.level=info "sj://$BUCKET" $MOUNT_DIR &

MOUNT_PID=$!

kill -0 $MOUNT_PID

if [ $? -ne 0 ]
then
    echo "storj-mount process is not running"
    exit 1
fi

# TODO find why sleep is needed
sleep 3

cp -v "$SRC_DIR/big-upload-testfile" "$MOUNT_DIR"

sleep 1

cp -v "$MOUNT_DIR/big-upload-testfile" "$DST_DIR"

rm -v "$MOUNT_DIR/big-upload-testfile"

if cmp "$SRC_DIR/big-upload-testfile" "$DST_DIR/big-upload-testfile"
then
    echo "big upload testfile matches uploaded file"
else
    echo "big upload testfile does not match uploaded file"
fi

kill -SIGTERM $MOUNT_PID

# wait for storj-mount process to be finished
while ps -p $MOUNT_PID > /dev/null; do sleep 1; done;

# verify that operations were done on mounted dir, not just file system
if [ "$(ls -A $MOUNT_DIR)" ]; then
    echo "storj-mount was not started, $MOUNT_DIR is not empty"
    exit 1
fi
