#!/usr/bin/env bash
set -eu
set -o pipefail

echo -n "Build timestamp: "
TIMESTAMP=$(date +%s)
echo $TIMESTAMP

echo -n "Git commit: "
if [[ "$(git diff --stat)" != '' ]] || [[ -n "$(git status -s)" ]]; then
  COMMIT=$(git rev-parse HEAD)-dirty
else
  COMMIT=$(git rev-parse HEAD)
fi
echo $COMMIT

echo -n "Tagged version: "
if git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*"; then
  VERSION=$(git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*")
else
  VERSION=$(git show -s --date='format:%Y.%m' --format='v%cd.%ct-%h' HEAD)
fi
echo $VERSION

echo Running "go $@"
exec go "$1" -ldflags \
  "-X storj.io/common/version.buildTimestamp=$TIMESTAMP
   -X storj.io/common/version.buildCommitHash=$COMMIT
   -X storj.io/common/version.buildVersion=$VERSION
   -X storj.io/common/version.buildRelease=true" "${@:2}"
