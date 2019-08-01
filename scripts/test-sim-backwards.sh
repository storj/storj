#!/usr/bin/env bash
set -ueo pipefail
set +x

BRANCH_DIR="$(pwd)/branch"
RELEASE_DIR="$(pwd)/release"
latestReleaseTag=$(git describe --tags `git rev-list --tags --max-count=1`)
latestReleaseCommit=$(git rev-list -n 1 "$latestReleaseTag")
echo "Checking out latest release tag: $latestReleaseTag"
# git worktree add -f "$RELEASE_DIR" "$latestReleaseCommit"

git worktree add -f "$BRANCH_DIR" c9306e774ee972eb1d87f8f5a6b2ac8dcfe96a34

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

: "${RELEASE_DIR?Environment variable RELEASE_DIR needs to be set}"

# make -C "$RELEASE_DIR" install-sim
cd "$BRANCH_DIR"
go install ./...
cd ..

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}

# setup the network
storj-sim -x --host $STORJ_NETWORK_HOST4 network setup

# run upload/download backward compatibility tests for last release branch
# and master branch
storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh upload

# make -C "$BRANCH_DIR" install-sim
# make -C "$SCRIPTDIR"/.. install-sim

storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

storj-sim -x --host $STORJ_NETWORK_HOST4 network destroy
