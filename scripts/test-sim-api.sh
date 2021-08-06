#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
export REPOROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. >/dev/null 2>&1 && pwd )"
TESTDIR="${REPOROOT}/web/satellite/tests/graphql"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

echo "Running test-sim-api"
echo "Directory Variables:"
echo "SCRIPTDIR: ${SCRIPTDIR}"
echo "REPOROOT: ${REPOROOT}"
echo "TESTDIR: ${TESTDIR}"

echo "Make install-sim"
make -C "$SCRIPTDIR"/.. install-sim

echo "putting tmp in path and setting storj network directory to tmp directory."
export PATH=$TMP:$PATH
export STORJ_NETWORK_DIR=$TMP

echo "setting network host 4 only when its unset."
STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.7}

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$TESTDIR"/test_graphql.sh
