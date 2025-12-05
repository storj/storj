#! /usr/bin/env bash
set -Eeo pipefail
set +x

if [ -z "${STORJ_REDIS_HOST}" ]; then
	echo STORJ_REDIS_HOST env var is required
	exit 1
fi

if [ -z "${STORJ_REDIS_PORT}" ]; then
	echo STORJ_REDIS_PORT env var is required
	exit 1
fi

if [ -z "${STORJ_REDIS_DIR}" ]; then
	echo STORJ_REDIS_DIR env var is required
	exit 1
fi

start() {
	if [ -f "${STORJ_REDIS_DIR}/redis-server.pid" ]; then
		return
	fi

	if [ ! -f "${STORJ_REDIS_DIR}/redis.conf" ]; then
		cat >>"${STORJ_REDIS_DIR}/redis.conf" <<EOF
bind ${STORJ_REDIS_HOST}
port ${STORJ_REDIS_PORT}
timeout 0
databases 2
dbfilename redis.db
dir ${STORJ_REDIS_DIR}
daemonize yes
loglevel warning
logfile ${STORJ_REDIS_DIR}/redis-server.log
pidfile ${STORJ_REDIS_DIR}/redis-server.pid
EOF
	fi

	redis-server "${STORJ_REDIS_DIR}/redis.conf"
}

stop() {
	# if the file exists, then Redis should be running
	if [ -f "${STORJ_REDIS_DIR}/redis-server.pid" ]; then
		if ! redis-cli -h "${STORJ_REDIS_HOST}" -p "${STORJ_REDIS_PORT}" shutdown; then
			echo "******************************** REDIS SERVER LOG (last 25 lines) ********************************"
			echo "Printing the last 25 lines"
			echo
			tail -n 25 "${STORJ_REDIS_DIR}/redis-server.log" || true
			echo
			echo "************************************** END REDIS SERVER LOG **************************************"
		fi
	fi
}

case "${1}" in
start) start ;;
stop) stop ;;
*) echo "the script must be executed as: $(basename "${BASH_SOURCE[0]}") start | stop" ;;
esac
