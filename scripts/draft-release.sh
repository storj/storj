#!/usr/bin/env bash
# usage: $0 vMAJOR.MINOR.PATCH[-rc[-*]] PATH/TO/BINARIES

set -euo pipefail

apps="identity uplink storagenode multinode"

TAG="${1-}"

if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc+(-.*)?)?$ ]]; then
  echo "No tag detected, skipping release drafting"
  exit 0
fi

FOLDER="${2-}"

echo "Drafting release"
github-release release --user storj --repo storj --tag "$TAG" --draft

echo "Sleep 10 seconds in order to wait for release propagation"
sleep 10

echo "Uploading binaries to release draft"
for app in $apps;
do
  for file in "$FOLDER/$app"*.zip
  do
    github-release upload --user storj --repo storj --tag "$TAG" --name $(basename "$file") --file "$file"
  done
done
echo "Drafting release done"
