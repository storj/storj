#!/bin/bash

# NOTE this script MUST BE EXECUTED from the same directory where it's located
# to always obtain the same paths in the satellite configuration file.

set -ueo pipefail

read -p "Have you warned the DevOps Team before updating this file? " -n 1 -r
echo    # (optional) move to a new line
if [[ ! $REPLY =~ ^[Yy]$ ]]
then
  echo operation aborted!!!
  exit 1
fi

#setup tmpdir for testfiles and cleanup
TMP_DIR=$(mktemp -d -t update-satellite-cfg-lock-XXXXX)
cleanup(){
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

go build -o "$TMP_DIR/satellite" "../cmd/satellite"

PATH=$TMP_DIR:$PATH
TESTDATA_DIR="./testdata"
satellite --config-dir "$TESTDATA_DIR" --defaults release setup > /dev/null
mv "$TESTDATA_DIR/config.yaml" "$TESTDATA_DIR/satellite-config.yaml.lock"
