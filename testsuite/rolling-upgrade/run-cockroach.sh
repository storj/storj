#!/bin/bash

set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$SCRIPTDIR/../.."

# Cleanup after we exit. If this doesn't run then "--renew-anon-volumes" will
# also cleanup the previous state.
trap 'docker compose -f ./testsuite/rolling-upgrade/docker-compose-cockroach.yaml down' EXIT

# We need to fetch all tags and the main branch to ensure we have the latest changes.
# This is done before docker compose so the image copy includes all refs needed by start-sim.sh.
# The script uses git worktree to checkout release tags, so we need all tag objects locally.
git fetch --tags --progress -- https://github.com/storj/storj.git +refs/heads/main:refs/remotes/origin/main

# Run the tests inside a docker compose environment.
docker compose -f testsuite/rolling-upgrade/docker-compose-cockroach.yaml up \
	--build \
	--abort-on-container-exit \
	--renew-anon-volumes \
	--exit-code-from test-runner
