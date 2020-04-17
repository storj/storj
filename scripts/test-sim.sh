#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

echo "Overriding default max segment size to 6MiB"
GOBIN=$TMP go install -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink

# use modifed version of uplink
export PATH=$TMP:$PATH

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# run aws-cli tests
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-uplink.sh
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-uplink-share.sh
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy

# setup the network with ipv6
#storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
#find "$STORJ_NETWORK_DIR"/gateway -type f -name config.yaml -exec sed -i 's/server.address: "\[::1\]/server.address: "127.0.0.1/' '{}' +
# run aws-cli tests using ipv6
#storj-sim -x --host "::1" network test bash "$SCRIPTDIR"/test-sim-aws.sh
#storj-sim -x network destroy
