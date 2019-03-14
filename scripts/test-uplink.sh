#!/bin/bash
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

mkdir -p $SRC_DIR $DST_DIR

head -c 1024 </dev/urandom > $SRC_DIR/small-upload-testfile      # create 1mb file of random bytes (inline)
head -c 5120 </dev/urandom > $SRC_DIR/big-upload-testfile        # create 5mb file of random bytes (remote)
head -c 5    </dev/urandom > $SRC_DIR/multipart-upload-testfile  # create 5kb file of random bytes (remote)

uplink --config-dir $GATEWAY_0_DIR mb sj://$BUCKET/ 

uplink --config-dir $GATEWAY_0_DIR cp $SRC_DIR/small-upload-testfile sj://$BUCKET/ 
uplink --config-dir $GATEWAY_0_DIR cp $SRC_DIR/big-upload-testfile sj://$BUCKET/ 
uplink --config-dir $GATEWAY_0_DIR cp $SRC_DIR/multipart-upload-testfile sj://$BUCKET/ 

uplink --config-dir $GATEWAY_0_DIR cp sj://$BUCKET/small-upload-testfile $DST_DIR
uplink --config-dir $GATEWAY_0_DIR cp sj://$BUCKET/big-upload-testfile $DST_DIR
uplink --config-dir $GATEWAY_0_DIR cp sj://$BUCKET/multipart-upload-testfile $DST_DIR

uplink --config-dir $GATEWAY_0_DIR rm sj://$BUCKET/small-upload-testfile
uplink --config-dir $GATEWAY_0_DIR rm sj://$BUCKET/big-upload-testfile
uplink --config-dir $GATEWAY_0_DIR rm sj://$BUCKET/multipart-upload-testfile

uplink --config-dir $GATEWAY_0_DIR ls sj://$BUCKET

uplink --config-dir $GATEWAY_0_DIR rb sj://$BUCKET

if cmp $SRC_DIR/small-upload-testfile $DST_DIR/small-upload-testfile
then
    echo "small upload testfile matches uploaded file"
else
    echo "small upload testfile does not match uploaded file"
fi

if cmp $SRC_DIR/big-upload-testfile $DST_DIR/big-upload-testfile
then
    echo "big upload testfile matches uploaded file"
else
    echo "big upload testfile does not match uploaded file"
fi

if cmp $SRC_DIR/multipart-upload-testfile $DST_DIR/multipart-upload-testfile
then
    echo "multipart upload testfile matches uploaded file"
else
    echo "multipart upload testfile does not match uploaded file"
fi

# check if all data files were removed
FILES=$(find "$STORAGENODE_0_DIR/../" -type f -path "*/blob/*" ! -name "info.*")
if [ -z "$FILES" ];
then
    echo "all data files removed from storage nodes"
else
    echo "not all data files removed from storage nodes:"
    echo $FILES
    exit 1
fi