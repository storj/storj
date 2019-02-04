#!/bin/bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

make -C $SCRIPTDIR/.. install-sim

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

export STORJ_NETWORK_DIR=$TMP

# setup the network
storj-sim -x network setup

# run aws-cli tests
storj-sim -x network test bash $SCRIPTDIR/test-sim-aws.sh
storj-sim -x network destroy

# setup the network with ipv6
storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
find $STORJ_NETWORK_DIR/gateway -type f -name config.yaml -exec sed -i "s/server.address: \"\[::1\]/server.address: \"127.0.0.1/" {} +
# run aws-cli tests using ipv6
storj-sim -x --host "::1" network test bash $SCRIPTDIR/test-sim-aws.sh
storj-sim -x network destroy