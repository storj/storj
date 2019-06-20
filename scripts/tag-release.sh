#!/usr/bin/env bash

set -eu
set -o pipefail

VERSION="$1"
shift

if [[ "$VERSION" == "" ]]; then
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
