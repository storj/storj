#!/usr/bin/env bash

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
export STORJ_REDIS_PORT=7379

# shellcheck source=/postgres-dev.sh
source "${SCRIPTDIR}/../postgres-dev.sh"

"${SCRIPTDIR}/start-sim.sh"
