#!/usr/bin/env bash
set -ueo pipefail
set +x

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
export STORJ_NETWORK_DIR=$TMP
cleanup(){
    rm -rf "$STORJ_NETWORK_DIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

BRANCH_DIR="$STORJ_NETWORK_DIR/branch"
RELEASE_DIR="$STORJ_NETWORK_DIR/release"

# setup two different directories containing the code for the latest release tag
# and for the current branch code
git worktree add -f "$BRANCH_DIR"

latestReleaseTag=$(git describe --tags `git rev-list --tags --max-count=1`)
latestReleaseCommit=$(git rev-list -n 1 "$latestReleaseTag")
echo "Checking out latest release tag: $latestReleaseTag"
git worktree add -f "$RELEASE_DIR" "$latestReleaseCommit"
# to run with sqlite, we need to delete this release file that forces postgres
rm "$RELEASE_DIR/internal/version/release.go"

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

make -C "$RELEASE_DIR" install-sim

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}

# setup the network
storj-sim -x --host $STORJ_NETWORK_HOST4 network setup

# run upload part of backward compatibility tests from the lastest release branch
storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh upload

make -C "$BRANCH_DIR" install-sim

# this replaces anywhere that has "/release/" in the config file, which currently just renames the static dir paths
sed -i -e 's#/release/#/branch/#g' $STORJ_NETWORK_DIR/satellite/0/config.yaml

# run download part of backward compatibility tests from the current branch
storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

storj-sim -x --host $STORJ_NETWORK_HOST4 network destroy
