#!/usr/bin/env bash
set -xueo pipefail

##
## Set up temporary directories, environment variables, and helper functions
##

STORJ_NUM_NODES=10
STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

if [ -z "${STORJ_SIM_POSTGRES}" ]; then
    echo "Postgres is required for the satellite DB. Exiting."
    exit 1
fi

STORJ_NETWORK_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
export STORJ_NETWORK_DIR

cleanup() {
    git worktree remove -f "$RELEASE_DIR"
    git worktree remove -f "$BRANCH_DIR"
    rm -rf "$STORJ_NETWORK_DIR"
}
trap cleanup EXIT

BRANCH_DIR="$STORJ_NETWORK_DIR/branch"
RELEASE_DIR="$STORJ_NETWORK_DIR/release"

test() {
    DIR=$1
    shift

    PATH="$DIR"/bin:"$PATH" storj-sim -x --storage-nodes="$STORJ_NUM_NODES" --host="$STORJ_NETWORK_HOST4" network test -- bash "$SCRIPTDIR"/steps.sh "$@"
}

test_release() {
    test "$RELEASE_DIR" "$@"
}

test_branch() {
    test "$BRANCH_DIR" "$@"
}

install_sim_noquic(){
    local bin_dir="$1"
    mkdir -p ${bin_dir}

    go build -race -tags noquic -o ${bin_dir}/storagenode storj.io/storj/cmd/storagenode 2>&1
    go build -race -tags noquic -o ${bin_dir}/satellite storj.io/storj/cmd/satellite 2>&1
    go build -race -tags noquic -o ${bin_dir}/storj-sim storj.io/storj/cmd/storj-sim 2>&1
    go build -race -tags noquic -o ${bin_dir}/versioncontrol storj.io/storj/cmd/versioncontrol 2>&1

    go build -race -tags noquic -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1
    go build -race -tags noquic -o ${bin_dir}/identity storj.io/storj/cmd/identity 2>&1
    go build -race -tags noquic -o ${bin_dir}/certificates storj.io/storj/cmd/certificates 2>&1

    GOBIN=${bin_dir} go install -race -tags noquic storj.io/gateway@latest
}

##
## Build the release and branch binaries and set up the network
##

# setup two different directories containing the code for the latest release tag
# and for the current branch code
git worktree add -f "$BRANCH_DIR" HEAD

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

latestReleaseTag=$($SCRIPTDIR/../find-previous-release.sh)
latestReleaseCommit=$(git rev-list -n1 $latestReleaseTag)

echo "Checking out latest release tag: $latestReleaseTag"
git worktree add -f "$RELEASE_DIR" "$latestReleaseCommit"

# delete this file that forces production config settings
rm -f "$RELEASE_DIR"/internal/version/release.go

# clear out release information
cat > "$RELEASE_DIR"/private/version/release.go <<EOF
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version
EOF

pushd $RELEASE_DIR
    install_sim_noquic "$RELEASE_DIR"/bin
popd

GOBIN="$BRANCH_DIR"/bin  make -C "$BRANCH_DIR" install-sim

echo "Overriding default max segment size to 6MiB"
pushd $RELEASE_DIR
    GOBIN=$RELEASE_DIR/bin go install -tags noquic -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink
popd
pushd $BRANCH_DIR
    GOBIN=$BRANCH_DIR/bin go install -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink
popd

# setup the network using the release
PATH="$RELEASE_DIR"/bin:"$PATH" storj-sim -x --host "$STORJ_NETWORK_HOST4" network --postgres="$STORJ_SIM_POSTGRES" setup

##
## Run some basic tests on the release branch, creating data for later tests.
##

# upload using everything release
test_release -b release-network-release-uplink upload

# check that it worked with everything release
test_release -b release-network-release-uplink download

##
## Change a bunch of settings to run on the current branch
##

SATELLITE_CONFIG="$(storj-sim network env SATELLITE_0_DIR)"/config.yaml

# this replaces anywhere that has "/release/" in the config file, which currently just renames the static dir paths
sed -i -e 's#/release/#/branch/#g' "$SATELLITE_CONFIG"

# replace any 140XX port with 100XX port to fix, satellite.API part removal from satellite.Core
sed -i -e "s#$STORJ_NETWORK_HOST4:140#$STORJ_NETWORK_HOST4:100#g" "$SATELLITE_CONFIG"

# add new address for admin panel
if ! grep -q "admin.address" "$SATELLITE_CONFIG"; then
    echo admin.address: "$STORJ_NETWORK_HOST4":10005 >> "$SATELLITE_CONFIG"
fi

# create redis config if it's missing
REDIS_CONFIG=$(storj-sim network env REDIS_0_DIR)/redis.conf
if [ ! -f "$REDIS_CONFIG" ] ; then
    {
        echo "daemonize no"
        echo "bind $STORJ_NETWORK_HOST4"
        echo "port 10004"
        echo "timeout 0"
        echo "databases 2"
        echo "dbfilename sim.rdb"
        echo "dir ./"
    } >> "$REDIS_CONFIG"
fi

# setup multinode if config is missing
MULTINODE_DIR=$(storj-sim network env MULTINODE_0_DIR)
if [ ! -f "$MULTINODE_DIR/config.yaml" ]; then
    multinode $(storj-sim --host "$STORJ_NETWORK_HOST4" network env MULTINODE_0_SETUP_ARGS)
fi

# keep half of the storage nodes on the old version
ln "$RELEASE_DIR"/bin/storagenode "$(storj-sim network env STORAGENODE_0_DIR)"/storagenode
ln "$RELEASE_DIR"/bin/storagenode "$(storj-sim network env STORAGENODE_1_DIR)"/storagenode
ln "$RELEASE_DIR"/bin/storagenode "$(storj-sim network env STORAGENODE_2_DIR)"/storagenode
ln "$RELEASE_DIR"/bin/storagenode "$(storj-sim network env STORAGENODE_3_DIR)"/storagenode
ln "$RELEASE_DIR"/bin/storagenode "$(storj-sim network env STORAGENODE_4_DIR)"/storagenode

# upgrade the trust configuration on the other half as the old configuration is
# most certainly not being used outside of test environments and is not
# backwards compatible (i.e. ignored)
sed -i -e "s#storage.whitelisted-satellites#storage2.trust.sources#g" "$(storj-sim network env STORAGENODE_5_DIR)"/config.yaml
sed -i -e "s#storage.whitelisted-satellites#storage2.trust.sources#g" "$(storj-sim network env STORAGENODE_6_DIR)"/config.yaml
sed -i -e "s#storage.whitelisted-satellites#storage2.trust.sources#g" "$(storj-sim network env STORAGENODE_7_DIR)"/config.yaml
sed -i -e "s#storage.whitelisted-satellites#storage2.trust.sources#g" "$(storj-sim network env STORAGENODE_8_DIR)"/config.yaml
sed -i -e "s#storage.whitelisted-satellites#storage2.trust.sources#g" "$(storj-sim network env STORAGENODE_9_DIR)"/config.yaml

# For cases where the release predates changeset I0e7e92498c3da768df5b4d5fb213dcd2d4862924,
# adjust all last_net values for future compatibility. this migration step is only necessary for
# satellites which existed before the aforementioned changeset and use dev defaults (to be specific,
# DistinctIP is off). This is a harmless change for any other satellites using dev defaults.
if [ "${STORJ_SIM_POSTGRES#cockroach:}" != "$STORJ_SIM_POSTGRES" ]; then
    schema_set=
    pgurl="${STORJ_SIM_POSTGRES/cockroach:/postgres:}"
    pgurl="${pgurl%?sslmode=disable}/satellite/0?sslmode=disable"
else
    schema_set='set search_path to "satellite/0"; '
    pgurl="$STORJ_SIM_POSTGRES"
fi
psql "$pgurl" -c "${schema_set}update nodes set last_net = last_ip_port"

# Run with 9 nodes to exercise more code paths with one node being offline.
STORJ_NUM_NODES=9

##
## Run tests on the branch under test.
##

# check that branch uplink + branch network can read fully release data
test_branch -b release-network-release-uplink download

# check that branch uplink + branch network can upload
test_branch -b branch-network-branch-uplink upload

##
## Run even more tests with the old uplink binary.
##

# overwrite new uplink with release branch and test the download
cp "$RELEASE_DIR"/bin/uplink "$BRANCH_DIR"/bin/uplink

# check that release uplink + branch network can read fully release data
test_branch -b release-network-release-uplink download

# check that release uplink + branch network can read fully branch data
test_branch -b branch-network-branch-uplink download

# check that release uplink + branch network can upload
test_branch -b branch-network-release-uplink upload

# check that release uplink + branch network can read mixed data
test_branch -b branch-network-release-uplink download

##
## Perform cleanup, deleting all of the files/buckets.
##

test_branch cleanup
