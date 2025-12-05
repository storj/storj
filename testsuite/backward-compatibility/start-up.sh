#!/usr/bin/env bash
set -xueo pipefail

GOOS=$(go env GOOS)
if { [ "$GOOS" != "linux" ]; } && { [ -z "${STORJ_CC_TARGET:-}" ] || [ -z "${STORJ_CXX_TARGET:-}" ]; }
then
  echo "Both STORJ_CC_TARGET and STORJ_CXX_TARGET must be set when building for non-linux platforms. Exiting..."
  exit 1
fi

DB=${1:-}

case "$DB" in
    'postgres') echo "running test with postgres DB"
        ;;
    'cockroach') echo "running test with cockroach DB"
        ;;
    *) echo "invalid DB specified, defaulting to cockroach"
      DB="cockroach"
        ;;
esac

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
export STORJ_NETWORK_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
  if [ -f "$STORJ_NETWORK_DIR"/docker-compose.yaml ]
  then
    docker compose -f "$STORJ_NETWORK_DIR"/docker-compose.yaml down
  fi
  rm -rf "$STORJ_NETWORK_DIR"
}
trap cleanup EXIT

#### build release binaries ####
RELEASE_BIN="$STORJ_NETWORK_DIR/bin/release"
# replace this with a standard go install once go allows install cross-compiled binaries when GOBIN is set
# https://github.com/golang/go/issues/57485
git worktree add -f "$STORJ_NETWORK_DIR"/branch HEAD

latestReleaseTag=$($SCRIPTDIR/../find-previous-release.sh)
latestReleaseCommit=$(git rev-list -n1 $latestReleaseTag)

echo "Checking out latest release tag: $latestReleaseTag"
git worktree add -f "$STORJ_NETWORK_DIR"/release "$latestReleaseCommit"
pushd "$STORJ_NETWORK_DIR"/release
  case $GOOS in
    linux)    GOOS=linux GOARCH=$(go env GOARCH) go build -tags noquic -o "$RELEASE_BIN"/storagenode -v storj.io/storj/cmd/storagenode   ;;
    *)        CGO_ENABLED=1 GOOS=linux GOARCH=$(go env GOARCH) CC=$STORJ_CC_TARGET CXX=$STORJ_CXX_TARGET go build -tags noquic -o "$RELEASE_BIN"/storagenode -v storj.io/storj/cmd/storagenode ;;
  esac
  GOOS=linux GOARCH=$(go env GOARCH) go build -tags noquic -o "$RELEASE_BIN"/satellite -v storj.io/storj/cmd/satellite
  GOOS=$GOOS GOARCH=$(go env GOARCH) go build -tags noquic -o "$RELEASE_BIN"/uplink -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink
popd

go install storj.io/storj-up@latest

#### setup the release network ####
cd "$STORJ_NETWORK_DIR"
if [ "$DB" == "cockroach" ]
then
  storj-up init minimal,db
else
  storj-up init minimal,redis
  storj-up add postgres
  storj-up port remove postgres 5432
  storj-up port add postgres 6543
  storj-up env set postgres PGPORT=6543
  storj-up env set satellite-api STORJ_DATABASE=postgres://postgres@postgres:6543/master?sslmode=disable
  storj-up env set satellite-api STORJ_METAINFO_DATABASE_URL=postgres://postgres@postgres:6543/master?sslmode=disable
fi
storj-up env set satellite-api STORJ_DATABASE_OPTIONS_MIGRATION_UNSAFE="full"
storj-up local satellite-api,storagenode -d "$RELEASE_BIN"
# persist the 5 nodes that will be restarted with branch binaries
mkdir -p {storagenode1/storj,storagenode2/storj,storagenode3/storj,storagenode4/storj,storagenode5/storj}
storj-up persist storagenode1,storagenode2,storagenode3,storagenode4,storagenode5

# start the services
docker compose up -d
if [ "$DB" == "cockroach" ]
then
  storj-up health -d 90
else
  storj-up health -d 90 -u postgres -p 6543
fi
eval $(storj-up credentials -e)

#### release tests ####
# upload using everything release
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b release-network-release-uplink upload

# check that it worked with everything release
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b release-network-release-uplink download

#### build branch binaries ####
BRANCH_BIN="$STORJ_NETWORK_DIR/bin/branch"
cd "$SCRIPTDIR"
  case $GOOS in
    linux)    GOBIN=$BRANCH_BIN GOOS=linux GOARCH=$(go env GOARCH) go install -race storj.io/storj/cmd/storagenode 2>&1   ;;
    *)        CGO_ENABLED=1  GOBIN=$BRANCH_BIN GOOS=linux GOARCH=$(go env GOARCH) CC=$STORJ_CC_TARGET CXX=$STORJ_CXX_TARGET go install -race storj.io/storj/cmd/storagenode 2>&1 ;;
  esac
GOBIN=$BRANCH_BIN GOOS=linux GOARCH=$(go env GOARCH) go install -race storj.io/storj/cmd/satellite 2>&1
GOBIN=$BRANCH_BIN GOOS=$GOOS GOARCH=$(go env GOARCH) go install -race -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink 2>&1

#### setup the branch network ####
cd "$STORJ_NETWORK_DIR"
# Kill 1 node to run with 9 nodes and exercise more code paths with one node being offline.
docker compose rm -sv storagenode10
# update satellite and 5 storage nodes to use branch binaries.
storj-up local satellite-api,storagenode1,storagenode2,storagenode3,storagenode4,storagenode5 -d $BRANCH_BIN
# start the branch services
docker compose up -d satellite-api storagenode1 storagenode2 storagenode3 storagenode4 storagenode5
# wait for branch network to be ready
docker compose exec -T storagenode1 storj-up util wait-for-satellite satellite-api:7777
docker compose exec -T satellite-api storj-up util wait-for-port storagenode1:30001
docker compose exec -T satellite-api storj-up util wait-for-port storagenode2:30011
docker compose exec -T satellite-api storj-up util wait-for-port storagenode3:30021
docker compose exec -T satellite-api storj-up util wait-for-port storagenode4:30031
docker compose exec -T satellite-api storj-up util wait-for-port storagenode5:30041

# todo: replace with a proper health check
sleep 60

#### Branch tests ####
# check that branch uplink + branch network can read fully release data
PATH="$BRANCH_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b release-network-release-uplink download

# check that branch uplink + branch network can upload
PATH="$BRANCH_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b branch-network-branch-uplink upload

# check that release uplink + branch network can read fully release data
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b release-network-release-uplink download

# check that release uplink + branch network can read fully branch data
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b branch-network-branch-uplink download

# check that release uplink + branch network can upload
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b branch-network-release-uplink upload

# check that release uplink + branch network can read mixed data
PATH="$RELEASE_BIN":"$PATH" "$SCRIPTDIR""/steps.sh" -b branch-network-release-uplink download
