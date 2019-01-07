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

# setup the network
storj-sdk -config-dir $TMP -x network setup

# run aws-cli tests
storj-sdk -config-dir $TMP -x network test bash $SCRIPTDIR/test-storj-sdk-aws.sh
storj-sdk -config-dir $TMP -x network destroy

# setup the network with ipv6
storj-sdk -config-dir $TMP -x --host "::1" network setup
# run aws-cli tests using ipv6
storj-sdk -config-dir $TMP -x --host "::1" network test bash $SCRIPTDIR/test-storj-sdk-aws.sh
storj-sdk -config-dir $TMP -x network destroy