#!/usr/bin/env bash

# NOTE this script MUST BE EXECUTED from the same directory where it's located
# to always obtain the same paths in the satellite configuration file.

set -uo pipefail

#setup tmpdir for testfiles and cleanup
TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMPDIR"
}
trap cleanup EXIT

satellite --config-dir "$TMPDIR" --defaults release setup > /dev/null

diff "./testdata/satellite-config.yaml.lock" "$TMPDIR/config.yaml"
if [[ $? != 0 ]]; then
    echo
    echo "NOTIFY the Devops and PM when this test fails so they can plan for changing it in the release process before fixing it to merge your PR."
    echo "Once you have notified them you can update the lock file through another Makefile target"
    echo
    exit 1
fi
