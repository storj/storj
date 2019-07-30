#!/usr/bin/env bash
set -ueo pipefail
set +x

export BRANCH_DIR="$(pwd)"
export RELEASE_DIR="$(pwd)/release"

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

: "${RELEASE_DIR?Environment variable RELEASE_DIR needs to be set}"

make -C "$RELEASE_DIR" install-sim

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}

# setup the network
storj-sim -x --host $STORJ_NETWORK_HOST4 network setup

# run upload/download backward compatibility tests for last release branch
# and master branch
storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh upload

# make -C "$BRANCH_DIR" install-sim
make -C "$SCRIPTDIR"/.. install-sim

storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

storj-sim -x --host $STORJ_NETWORK_HOST4 network destroy
