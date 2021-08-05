#!/usr/bin/env bash
#set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
REPOROOT= "$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && cd.. && pwd )"
TESTDIR="$REPOROOT/web/satellite/tests/graphql"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

echo "Running test-sim-api"
echo "Directory Variables:"
echo "$SCRIPTDIR"
echo "$REPOROOT"
echo "$TESTDIR"

echo "Make install-sim"
make -C "$SCRIPTDIR"/.. install-sim

# use modified version of uplink
echo "setting path"
export PATH=$TMP:$PATH

echo "setting storj network directory to create tmp directory."
export STORJ_NETWORK_DIR=$TMP

echo "setting network host 4 only when its unset."
STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.7}

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network && test bash "$TESTDIR"/test_graphql.sh