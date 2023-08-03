#!/usr/bin/env bash
set -Eeo pipefail
set +x

# Required environment variables
if [ -z "${STORJ_SIM_POSTGRES}" ]; then
	echo "STORJ_SIM_POSTGRES environment variable must be set to a non-empty string"
	exit 1
fi

if [ -z "${STORJ_REDIS_PORT}" ]; then
	echo STORJ_REDIS_PORT env var is required
	exit 1
fi

# constants
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
readonly SCRIPT_DIR
TMP_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
readonly TMP_DIR
STORJ_REDIS_DIR=$(mktemp -d -p /tmp test-sim-redis.XXXX)
readonly STORJ_REDIS_DIR
export STORJ_REDIS_DIR

cleanup() {
	trap - EXIT ERR

	"${SCRIPT_DIR}/redis-server.sh" stop
	rm -rf "${TMP_DIR}"
	rm -rf "${STORJ_REDIS_DIR}"
}
trap cleanup ERR EXIT

echo "install sim"
make -C "$SCRIPT_DIR"/../../.. install-sim

echo "overriding default max segment size to 6MiB"
GOBIN="${TMP_DIR}" go install -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink

# use modified version of uplink
export PATH="${TMP_DIR}:${PATH}"
export STORJ_NETWORK_DIR="${TMP_DIR}"

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
export STORJ_REDIS_HOST=${STORJ_NETWORK_HOST4}

# TODO remove when metainfo.server-side-copy-duplicate-metadata will be dropped
export STORJ_METAINFO_SERVER_SIDE_COPY_DUPLICATE_METADATA=true

# setup the network
"${SCRIPT_DIR}/redis-server.sh" start
storj-sim --failfast -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--postgres="${STORJ_SIM_POSTGRES}" --redis="${STORJ_REDIS_HOST}:${STORJ_REDIS_PORT}" setup

# run test that checks that the satellite runs when Redis is up and down
storj-sim --failfast -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--redis="127.0.0.1:6379" test bash "${SCRIPT_DIR}/test-uplink-redis-up-and-down.sh" "${REDIS_CONTAINER_NAME}"

# run test that checks that the satellite runs despite of not being able to connect to Redis
"${SCRIPT_DIR}/redis-server.sh" stop
storj-sim --failfast -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--redis="127.0.0.1:6379" test bash "${SCRIPT_DIR}/../integration/test-uplink.sh"
