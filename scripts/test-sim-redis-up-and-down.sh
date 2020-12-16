#!/usr/bin/env bash
set -Eeuo pipefail
set +x

# Required environment variables
if [ -z "${STORJ_SIM_POSTGRES}" ]; then
	echo "STORJ_SIM_POSTGRES environment variable must be set to a non-empty string"
	exit 1
fi

# constants
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
readonly SCRIPT_DIR
REDIS_CONTAINER_NAME=storj_sim_redis
readonly REDIS_CONTAINER_NAME
TMP_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
readonly TMP_DIR

# setup tmpdir for testfiles and cleanup
cleanup() {
	trap - EXIT

	rm -rf "${TMP_DIR}"
	docker container rm -f "${REDIS_CONTAINER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "install sim"
make -C "$SCRIPT_DIR"/.. install-sim

echo "overriding default max segment size to 6MiB"
GOBIN="${TMP_DIR}" go install -v -ldflags "-X 'storj.io/uplink.maxSegmentSize=6MiB'" storj.io/storj/cmd/uplink

# use modified version of uplink
export PATH="${TMP_DIR}:${PATH}"
export STORJ_NETWORK_DIR="${TMP_DIR}"

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}

redis_run() {
	local retries=10

	docker container run -d -p 6379:6379 --name "${REDIS_CONTAINER_NAME}" redis:5.0-alpine
	until docker container exec "${REDIS_CONTAINER_NAME}" redis-cli ping >/dev/null 2>&1 ||
		[ ${retries} -eq 0 ]; do
		echo "waiting for Redis server to be ready, $((retries--)) remaining attemps..."
		sleep 1
	done

	if [ ${retries} -eq 0 ]; then
		echo "aborting, Redis server is not ready after several retrials"
		exit 1
	fi
}

redis_stop() {
	docker container stop "${REDIS_CONTAINER_NAME}"
}

# setup the network
storj-sim -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--postgres="${STORJ_SIM_POSTGRES}" --redis="127.0.0.1:6379" setup

# run test that checks that the satellite runs when Redis is up and down
redis_run
storj-sim -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--redis="127.0.0.1:6379" test bash "${SCRIPT_DIR}/test-uplink-redis-up-and-down.sh" "${REDIS_CONTAINER_NAME}"

# run test that checks that the satellite runs despite of not being able to connect to Redis
redis_stop
storj-sim -x --satellites 1 --host "${STORJ_NETWORK_HOST4}" network \
	--redis="127.0.0.1:6379" test bash "${SCRIPT_DIR}/test-uplink.sh"
