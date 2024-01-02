#!/usr/bin/env bash

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

source $SCRIPTDIR/../postgres-dev.sh

export STORJ_MIGRATION_DB="${STORJ_SIM_POSTGRES}&options=--search_path=satellite/0/meta"

$SCRIPTDIR/start-sim.sh