#!/bin/bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

go install -race -v storj.io/storj/cmd/{storj-sdk,satellite,storagenode,uplink,gateway}

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

STORJ_LOCAL_NETWORK=$TMP

# setup the network
storj-sdk -x network setup
# run test-storj-sdk-aws.sh case
storj-sdk -x network test bash $SCRIPTDIR/test-storj-sdk-aws.sh
storj-sdk -x network destroy

# setup the network with ipv6
storj-sdk -x --host "::1" network setup
# run test-storj-sdk-aws.sh case
storj-sdk -x --host "::1" network test bash $SCRIPTDIR/test-storj-sdk-aws.sh