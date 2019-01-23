#!/bin/bash
set -ueo pipefail

#setup tmpdir for testfiles and cleanup
TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMPDIR"
}
trap cleanup EXIT

SRC_DIR=$TMPDIR/source
DST_DIR=$(mktemp -d -t tmp.XXXXXXXXXX)
mkdir -p $SRC_DIR $DST_DIR

payments generatecsv