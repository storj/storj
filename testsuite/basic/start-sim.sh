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

echo "Running basic/start-sim"
make -C "$SCRIPTDIR"/../.. install-sim

echo "Overriding default max segment size to 6MiB"
GOBIN=$TMP go install -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink

# use modified version of uplink
export PATH=$TMP:$PATH

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

# TODO remove when metainfo.server-side-copy-duplicate-metadata will be dropped
export STORJ_METAINFO_SERVER_SIDE_COPY_DUPLICATE_METADATA=true
# TODO remove when we get rid of this feature flag
export STORJ_CONSOLE_SIGNUP_ACTIVATION_CODE_ENABLED=false

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# run tests
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/step-uplink.sh
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/step-uplink-share.sh

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/step-uplink-rs-upload.sh
# change RS values and try download
sed -i 's@# metainfo.rs: 4/6/8/10-256 B@metainfo.rs: 2/3/6/8-256 B@g' $(storj-sim network env SATELLITE_0_DIR)/config.yaml
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/step-uplink-rs-download.sh

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy

# setup the network with ipv6
#storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
#find "$STORJ_NETWORK_DIR"/gateway -type f -name config.yaml -exec sed -i 's/server.address: "\[::1\]/server.address: "127.0.0.1/' '{}' +
# run aws-cli tests using ipv6
#storj-sim -x --host "::1" network test bash "$SCRIPTDIR"/step-sim-aws.sh
#storj-sim -x network destroy
