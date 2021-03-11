#!/usr/bin/env bash

SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# shellcheck source=/postgres-dev.sh
source "${SCRIPTDIR}/postgres-dev.sh"

"${SCRIPTDIR}/test-sim-redis-up-and-down.sh"
