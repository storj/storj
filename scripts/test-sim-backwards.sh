#!/usr/bin/env bash
set -ueo pipefail
set +x

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
export STORJ_NETWORK_DIR=$TMP
cleanup(){
    git worktree remove -f "$RELEASE_DIR"
    git worktree remove -f "$BRANCH_DIR"
    rm -rf "$STORJ_NETWORK_DIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

BRANCH_DIR="$STORJ_NETWORK_DIR/branch"
RELEASE_DIR="$STORJ_NETWORK_DIR/release"

# setup two different directories containing the code for the latest release tag
# and for the current branch code
git worktree add -f "$BRANCH_DIR" HEAD

latestReleaseTag=$(git describe --tags `git rev-list --tags --max-count=1`)
latestReleaseCommit=$(git rev-list -n 1 "$latestReleaseTag")
echo "Checking out latest release tag: $latestReleaseTag"
git worktree add -f "$RELEASE_DIR" "$latestReleaseCommit"
# delete this file that forces production config settings
rm "$RELEASE_DIR/internal/version/release.go"

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# replace unstable git.apache.org package with github
(cd $RELEASE_DIR && go mod edit -replace git.apache.org/thrift.git=github.com/apache/thrift@v0.12.0)

GOBIN=$RELEASE_DIR/bin make -C "$RELEASE_DIR" install-sim
GOBIN=$BRANCH_DIR/bin  make -C "$BRANCH_DIR" install-sim

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

if [ -z ${STORJ_SIM_POSTGRES} ]; then
    echo "Postgres is required for the satellite DB. Exiting."
    exit 1
fi

# setup the network
PATH=$RELEASE_DIR/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
# run upload part of backward compatibility tests from the lastest release branch
PATH=$RELEASE_DIR/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh upload

# this replaces anywhere that has "/release/" in the config file, which currently just renames the static dir paths
sed -i -e 's#/release/#/branch/#g' `storj-sim network env SATELLITE_0_DIR`/config.yaml

## Ensure that partially upgraded network works

# keep half of the storage nodes on the old version
ln $RELEASE_DIR/bin/storagenode `storj-sim network env STORAGENODE_0_DIR`/storagenode
ln $RELEASE_DIR/bin/storagenode `storj-sim network env STORAGENODE_1_DIR`/storagenode
ln $RELEASE_DIR/bin/storagenode `storj-sim network env STORAGENODE_2_DIR`/storagenode
ln $RELEASE_DIR/bin/storagenode `storj-sim network env STORAGENODE_3_DIR`/storagenode
ln $RELEASE_DIR/bin/storagenode `storj-sim network env STORAGENODE_4_DIR`/storagenode

# run download part of backward compatibility tests from the current branch, using new uplink
PATH=$BRANCH_DIR/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

## Ensure that old uplink works

# overwrite new uplink with release branch and test the download
cp $RELEASE_DIR/bin/uplink $BRANCH_DIR/bin/uplink

# run download part of backward compatibility tests from the current branch
PATH=$BRANCH_DIR/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

# run a delete in the network
PATH=$BRANCH_DIR/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh cleanup