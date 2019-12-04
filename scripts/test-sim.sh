#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# explicitly set all the satellites and storagenodes to use mixed grpc and drpc
(
	eval "$( storj-sim --satellites 2 network env )"

	N=0
	DIR="SATELLITE_${N}_DIR"
	while [ -n "${!DIR:-""}" ]; do
		[ $((N%2)) -eq 0 ] && KIND=drpc || KIND=grpc
		BIN="$(which satellite-${KIND})"
		( set -x; cp "${BIN}" "${!DIR}/satellite" )
		let N=N+1
		DIR="SATELLITE_${N}_DIR"
	done

	N=0
	DIR="STORAGENODE_${N}_DIR"
	while [ -n "${!DIR:-""}" ]; do
		[ $((N%2)) -eq 0 ] && KIND=drpc || KIND=grpc
		BIN="$(which storagenode-${KIND})"
		( set -x; cp "${BIN}" "${!DIR}/storagenode" )
		let N=N+1
		DIR="STORAGENODE_${N}_DIR"
	done
)

# set the segment size lower to make test run faster
echo client.segment-size: "6 MiB" >> `storj-sim network env GATEWAY_0_DIR`/config.yaml

# run aws-cli tests
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-sim-aws.sh
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-uplink.sh
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network destroy

# setup the network with ipv6
#storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
#find "$STORJ_NETWORK_DIR"/gateway -type f -name config.yaml -exec sed -i 's/server.address: "\[::1\]/server.address: "127.0.0.1/' '{}' +
# run aws-cli tests using ipv6
#storj-sim -x --host "::1" network test bash "$SCRIPTDIR"/test-sim-aws.sh
#storj-sim -x network destroy
