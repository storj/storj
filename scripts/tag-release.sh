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

if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc+(-.*)?)?$ ]]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH[-rc[-*]]"
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

if [[ "$(git diff --stat)" != '' ]] || [[ -n "$(git status -s)" ]]; then
  echo "git working tree unclean"
  exit 1
fi

TIMESTAMP=$(date +%s)
COMMIT=$(git rev-parse HEAD)

cat > ./private/version/release.go <<EOF
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import _ "unsafe" // needed for go:linkname

//go:linkname buildTimestamp storj.io/common/version.buildTimestamp
var buildTimestamp string = "$TIMESTAMP"

//go:linkname buildCommitHash storj.io/common/version.buildCommitHash
var buildCommitHash string = "$COMMIT"

//go:linkname buildVersion storj.io/common/version.buildVersion
var buildVersion string = "$VERSION"

//go:linkname buildRelease storj.io/common/version.buildRelease
var buildRelease string = "true"

// ensure that linter understands that the variables are being used.
func init() { use(buildTimestamp, buildCommitHash, buildVersion, buildRelease) }

func use(...interface{}) {}
EOF

gofmt -w -s ./private/version/release.go
go install ./private/version

git add ./private/version/release.go >/dev/null
git commit -m "release $VERSION" >/dev/null
if git tag $VERSION; then
  echo successfully created tag $VERSION
fi
git reset --hard $COMMIT >/dev/null
