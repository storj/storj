#!/usr/bin/env bash

## This script:
#
# 1) Makes sure the current git working tree is clean
# 2) Creates a release file that changes the build defaults to include
#    a timestamp, a commit hash, a version number, and set the release
#    flag to true.
# 3) commits that release file and tags it with the release version
# 4) resets the working tree back
#
# This script should be used instead of 'git tag' for Storj releases,
# so downstream users developing with Go 1.11+ style modules find code
# with our release defaults set instead of our dev defaults set.
#

set -eu
set -o pipefail

VERSION="${1-}"

if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH"
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

if [[ "$(git diff --stat)" != '' ]] || [[ -n "$(git status -s)" ]]; then
  echo "git working tree unclean"
  exit 1
fi

TIMESTAMP=$(date +%s)
COMMIT=$(git rev-parse HEAD)

cat > ./internal/version/release.go <<EOF
// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

func init() {
  buildTimestamp = "$TIMESTAMP"
  buildCommitHash = "$COMMIT"
  buildVersion = "$VERSION"
  buildRelease = "true"
}
EOF

gofmt -w -s ./internal/version/release.go
go install ./internal/version

git add ./internal/version/release.go >/dev/null
git commit -m "release $VERSION" >/dev/null
if git tag $VERSION; then
  echo successfully created tag $VERSION
fi
git reset --hard $COMMIT >/dev/null
