#!/bin/bash
set -ueo pipefail

uplink mb s3://test
uplink cp big-upload-testfile s3://test/small
