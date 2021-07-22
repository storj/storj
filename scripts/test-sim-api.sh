#!/usr/bin/env bash
set -ueo pipefail
set +x

function onExit {
    rm -rf "$TMP"
    if [ "$?" != "0" ]; then
        echo "Tests failed"
        # build failed, don't deploy
        exit 1;
    else
        echo "Tests passed"
        # deploy build
		exit 0
    fi
}

trap onExit EXIT

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
REPOROOT="$( cd "../$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
TESTDIR="$REPOROOT/web/satellite/tests/graphql/"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

# use modified version of uplink
export PATH=$TMP:$PATH

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.7}

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network test bash "$TESTDIR"/test_graphql.sh