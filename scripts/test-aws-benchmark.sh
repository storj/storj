#!/bin/bash
set -ueo pipefail

# Purpose: This script executes upload and download benchmark tests against aws s3 to compare with storj performance.
# Setup: Assumes the awscli is installed. Assumes $AWS_ACCESS_KEY_ID and $AWS_SECRET_ACCESS_KEY environment
#   variables are set with valid aws credentials with permissions to read/write to aws s3.
# Usage: from root of storj repo, run
#   $ ./scripts/test-aws-benchmark.sh

aws configure set aws_access_key_id     "$AWS_ACCESS_KEY_ID"
aws configure set aws_secret_access_key "$AWS_SECRET_ACCESS_KEY"
aws configure set default.region        us-east-1

# run s3-benchmark with aws s3
echo
echo "Executing s3-benchmark tests with aws s3 client..."
s3-benchmark --client=aws-cli --accesskey="$AWS_ACCESS_KEY_ID" --secretkey="$AWS_SECRET_ACCESS_KEY" --location="us-east-1" --s3-gateway="https://s3.amazonaws.com/"
