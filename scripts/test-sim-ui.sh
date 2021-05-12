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

export PATH=$TMP/bin:$PATH
echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}
STORJ_SIM_REDIS=${STORJ_SIM_REDIS:-""}

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# update satellite settings
SATELLITE_CONFIG="$(storj-sim network env SATELLITE_0_DIR)"/config.yaml
sed -i 's#console.static-dir: ""#console.static-dir: "'$SCRIPTDIR'/../web/satellite"#g' $SATELLITE_CONFIG

# run UI tests
echo "section tests start"
Xvfb -ac :99 -screen 0 1280x1024x16 & export DISPLAY=:99
pushd "$SCRIPTDIR/../web/satellite/"
npm install
popd

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network run &
go test "$SCRIPTDIR"/tests/uitests/.

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy
