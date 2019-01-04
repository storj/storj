#!/bin/bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

go install -v storj.io/storj/cmd/{storj-sdk,satellite,storagenode,uplink,gateway}

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

storj-sdk --config-dir $TMP network setup
storj-sdk --config-dir $TMP network test  bash $SCRIPTDIR/test-storj-sdk-aws.sh

rm -rf $TMP