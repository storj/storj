#!/bin/bash
set -euo pipefail


TMP_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
CMP_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
# Clean up what we might have done
cleanup(){
	echo ""
	echo ""
	echo ""
	echo "=> Testing finished, logs to follow"
	echo "=> Satellite logs"
	docker logs storj_satellite_1
	echo "=> Storagenode logs"
	docker logs storj_storagenode_1
	echo "=> Gateway logs"
	docker logs storj_gateway_1
	echo "=> Cleaning up"
	rm -rf "$TMP_DIR" "$CMP_DIR"
	# Hide any ERRORs and Faileds here as they are not relevant to the actual
	# errors and failures of this test.
	docker-compose down --rmi all 2>&1 | grep -v ERROR | grep -v Failed
}
trap cleanup EXIT

mkdir -p "$TMP_DIR"
mkdir -p "$CMP_DIR"

# Stand up production images in a local environment
docker-compose up -d satellite storagenode gateway

echo "=> Waiting for the gateway to be ready"
until docker logs storj_gateway_1 | grep -q Access; do
	sleep 2
done

# Extract the keys for AWS client
access_key_id="$(docker logs storj_gateway_1 2>/dev/null| awk '/Access/{print $3; exit}')"
secret_access_key="$(docker logs storj_gateway_1 2>/dev/null| awk '/Secret/{print $3; exit}')"

echo "=> Access Key: $access_key_id"
echo "=> Secret Key: $secret_access_key"
export AWS_ACCESS_KEY_ID="$access_key_id"
export AWS_SECRET_ACCESS_KEY="$secret_access_key"


aws configure set default.region us-east-1

echo "=> Making test files"
head -c 1024 </dev/urandom > "$TMP_DIR/small-upload-testfile" # create 1mb file of random bytes (inline)
head -c 5120 </dev/urandom > "$TMP_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)
head -c 5 </dev/urandom > "$TMP_DIR/multipart-upload-testfile"     # create 5kb file of random bytes (remote)

echo "=> Making bucket"
aws s3 --endpoint=http://localhost:7777/ mb s3://bucket

echo "=> Uploading test files"
aws configure set default.s3.multipart_threshold 1TB
aws s3 --endpoint=http://localhost:7777/ cp "$TMP_DIR/small-upload-testfile" s3://bucket/small-testfile
starttime="$(date +%s)"
while true; do
	if aws s3 --endpoint=http://localhost:7777/ cp "$TMP_DIR/big-upload-testfile" s3://bucket/big-testfile; then
		break
	fi
	echo "=> Large file failed, sleeping for a bit before trying again"
	sleep 1
	if [ $(( $starttime + 60 )) -lt $(date +%s) ]; then
		echo "=> Failed to upload big-testfile for over a minute!"
		exit 1
	fi
done

# Wait 5 seconds to trigger any error related to one of the different intervals
sleep 5

aws configure set default.s3.multipart_threshold 4KB
aws s3 --endpoint=http://localhost:7777/ cp "$TMP_DIR/multipart-upload-testfile" s3://bucket/multipart-testfile

echo "=> Listing bucket"
aws s3 --endpoint=http://localhost:7777/ ls s3://bucket
echo "=> Downloading test files"
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/small-testfile "$CMP_DIR/small-download-testfile"
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/big-testfile "$CMP_DIR/big-download-testfile"
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/multipart-testfile "$CMP_DIR/multipart-download-testfile"
echo "=> Removing bucket"
aws s3 --endpoint=http://localhost:7777/ rb s3://bucket --force

echo "=> Comparing test files downloaded with uploaded versions"
if cmp "$TMP_DIR/small-upload-testfile" "$CMP_DIR/small-download-testfile"
then
  echo "Downloaded file matches uploaded file"
else
  echo "Downloaded file does not match uploaded file"
  exit 1
fi

if cmp "$TMP_DIR/big-upload-testfile" "$CMP_DIR/big-download-testfile"
then
  echo "Downloaded file matches uploaded file"
else
  echo "Downloaded file does not match uploaded file"
  exit 1
fi

if cmp "$TMP_DIR/multipart-upload-testfile" "$CMP_DIR/multipart-download-testfile"
then
  echo "Downloaded file matches uploaded file"
else
  echo "Downloaded file does not match uploaded file"
  exit 1
fi
