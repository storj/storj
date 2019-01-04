#!/bin/bash
set -ueo pipefail

go install -v storj.io/storj/cmd/{storj-sdk,satellite,storagenode,uplink,gateway}

storj-sdk network setup
storj-sdk network test ./test-storj-upload.sh