#!/usr/bin/env bash
# usage: $0 vMAJOR.MINOR.PATCH[-rc[-*]] PATH/TO/BINARIES

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

apps="identity uplink storagenode multinode"

GIT_TAG="${1-}"

if ! [[ "$GIT_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+(-.*)?)?$ ]]; then
  echo "No tag detected, skipping release creation"
  exit 0
fi

FOLDER="${2-}"

FLAGS=""
if [[ "$GIT_TAG" =~ -rc[0-9]+ ]]; then
  FLAGS="--pre-release"
fi

echo "Creating release"
current_release_version=$(echo "$GIT_TAG" | cut -d '.' -f 1-2)
previous_release_version=$(git describe --tags $(git rev-list --exclude='*rc*' --exclude=$current_release_version* --tags --max-count=1))
changelog=$(python3 -W "ignore" scripts/release/changelog.py "$previous_release_version" "$GIT_TAG" 2>&1)
github-release release --user storj --repo storj --tag "$GIT_TAG" --description "$changelog" $FLAGS

echo "Sleep 10 seconds in order to wait for release propagation"
sleep 10

echo "Uploading binaries to the release"
for app in $apps; do
  for file in "$FOLDER/$app"*.zip; do
    github-release upload --user storj --repo storj --tag "$GIT_TAG" --name $(basename "$file") --file "$file"
  done
done
echo "Publishing release done"
