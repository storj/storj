#!/bin/bash
set -ueo pipefail

#setup tmpdir for testfiles and cleanup
TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMPDIR"
}
trap cleanup EXIT

SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst
mkdir -p "$SRC_DIR" "$DST_DIR"

/usr/local/bin/aws configure set aws_access_key_id     "$GATEWAY_0_ACCESS_KEY"
/usr/local/bin/aws configure set aws_secret_access_key "$GATEWAY_0_SECRET_KEY"
/usr/local/bin/aws configure set default.region        us-east-1

random_bytes_file () {
	size=$1
	output=$2
	dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

random_bytes_file 1x1024x1024 "$SRC_DIR/small-upload-testfile"     # create 1mb file of random bytes (inline)
random_bytes_file 5x1024x1024 "$SRC_DIR/big-upload-testfile"       # create 5mb file of random bytes (remote)
random_bytes_file      5x1024 "$SRC_DIR/multipart-upload-testfile" # create 5kb file of random bytes (remote)

echo "Creating Bucket"
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" mb s3://bucket

echo "Uploading Files"
/usr/local/bin/aws configure set default.s3.multipart_threshold 1TB
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp "$SRC_DIR/small-upload-testfile" s3://bucket/small-testfile
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp "$SRC_DIR/big-upload-testfile"   s3://bucket/big-testfile

# Wait 5 seconds to trigger any error related to one of the different intervals
sleep 5

echo "Uploading Multipart File"
/usr/local/bin/aws configure set default.s3.multipart_threshold 4KB
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp "$SRC_DIR/multipart-upload-testfile" s3://bucket/multipart-testfile

echo "Downloading Files"
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" ls s3://bucket
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp s3://bucket/small-testfile     "$DST_DIR/small-download-testfile"
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp s3://bucket/big-testfile       "$DST_DIR/big-download-testfile"
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" cp s3://bucket/multipart-testfile "$DST_DIR/multipart-download-testfile"
/usr/local/bin/aws s3 --endpoint="http://$GATEWAY_0_ADDR" rb s3://bucket --force

if cmp "$SRC_DIR/small-upload-testfile" "$DST_DIR/small-download-testfile"
then
	echo "small-upload-testfile file matches uploaded file";
else
	echo "small-upload-testfile file does not match uploaded file";
fi

if cmp "$SRC_DIR/big-upload-testfile" "$DST_DIR/big-download-testfile"
then
	echo "big-upload-testfile file matches uploaded file";
else
	echo "big-upload-testfile file does not match uploaded file";
fi

if cmp "$SRC_DIR/multipart-upload-testfile" "$DST_DIR/multipart-download-testfile"
then
	echo "multipart-upload-testfile file matches uploaded file";
else
	echo "multipart-upload-testfile file does not match uploaded file";
fi
