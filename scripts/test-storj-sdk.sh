#!/bin/bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

go install -race -v storj.io/storj/cmd/{storj-sdk,bootstrap,satellite,storagenode,uplink,gateway}

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

export STORJ_LOCAL_NETWORK=$TMP

# setup the network
storj-sdk -x network setup

# run aws-cli tests
storj-sdk -x network test bash $SCRIPTDIR/test-storj-sdk-aws.sh
storj-sdk -x network destroy

# ipv6 tests disabled because aws-cli doesn't seem to support connecting to ipv6 host
# # setup the network with ipv6
# storj-sdk -x --host "::1" network setup
# # run aws-cli tests using ipv6
# storj-sdk -x --host "::1" network test bash $SCRIPTDIR/test-storj-sdk-aws.sh
# storj-sdk -x network destroy