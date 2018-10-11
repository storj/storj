#!/bin/bash 
set -ueo pipefail
go install -v storj.io/storj/cmd/captplanet

captplanet setup --overwrite

# run captplanet for 5 seconds to reproduce kademlia problems. See V3-526
captplanet run &
CAPT_PID=$!
sleep 5
kill -9 $CAPT_PID

captplanet run &
CAPT_PID=$!

aws configure set aws_access_key_id insecure-dev-access-key
aws configure set aws_secret_access_key insecure-dev-secret-key
aws configure set default.region us-east-1

head -c 1024 </dev/urandom > ./small-upload-testfile # create 1mb file of random bytes (inline)
head -c 5120 </dev/urandom > ./big-upload-testfile   # create 5mb file of random bytes (remote)
head -c 5 </dev/urandom > ./multipart-upload-testfile     # create 5kb file of random bytes (remote)

aws s3 --endpoint=http://localhost:7777/ mb s3://bucket

aws configure set default.s3.multipart_threshold 1TB
aws s3 --endpoint=http://localhost:7777/ cp ./small-upload-testfile s3://bucket/small-testfile
aws s3 --endpoint=http://localhost:7777/ cp ./big-upload-testfile s3://bucket/big-testfile

aws configure set default.s3.multipart_threshold 4KB
aws s3 --endpoint=http://localhost:7777/ cp ./multipart-upload-testfile s3://bucket/multipart-testfile

aws s3 --endpoint=http://localhost:7777/ ls s3://bucket
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/small-testfile ./small-download-testfile
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/big-testfile ./big-download-testfile
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/multipart-testfile ./multipart-download-testfile
aws s3 --endpoint=http://localhost:7777/ rb s3://bucket --force

if cmp ./small-upload-testfile ./small-download-testfile
then
  echo "Downloaded file matches uploaded file";
else
  echo "Downloaded file does not match uploaded file";
  kill -9 $CAPT_PID
  exit 1;
fi

if cmp ./big-upload-testfile ./big-download-testfile
then
  echo "Downloaded file matches uploaded file";
else
  echo "Downloaded file does not match uploaded file";
  kill -9 $CAPT_PID
  exit 1;
fi

if cmp ./multipart-upload-testfile ./multipart-download-testfile
then
  echo "Downloaded file matches uploaded file";
else
  echo "Downloaded file does not match uploaded file";
  kill -9 $CAPT_PID
  exit 1;
fi

kill -9 $CAPT_PID

captplanet setup --listen-host ::1 --overwrite
captplanet run &
CAPT_PID=$!

aws s3 --endpoint=http://localhost:7777/ mb s3://bucket
aws s3 --endpoint=http://localhost:7777/ cp ./big-upload-testfile s3://bucket/big-testfile
aws s3 --endpoint=http://localhost:7777/ cp s3://bucket/big-testfile ./big-download-testfile-ipv6
aws s3 --endpoint=http://localhost:7777/ rb s3://bucket --force

if cmp ./big-upload-testfile ./big-download-testfile-ipv6
then
  echo "Downloaded ipv6 file matches uploaded file";
else
  echo "Downloaded ipv6 file does not match uploaded file";
  kill -9 $CAPT_PID
  exit 1;
fi

kill -9 $CAPT_PID

rm small-upload-testfile
rm big-upload-testfile
rm multipart-upload-testfile
rm big-download-testfile-ipv6
