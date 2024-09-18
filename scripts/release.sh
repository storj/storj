#!/usr/bin/env bash
set -eu
set -o pipefail

echo -n "Build timestamp: "
TIMESTAMP=$(date +%s)
echo $TIMESTAMP

echo -n "Git commit: "
if [[ "$(git diff --stat)" != '' ]] || [[ -n "$(git status -s)" ]]; then
  echo "Changes detected, building a development version"
  COMMIT=$(git rev-parse HEAD)-dirty
  RELEASE=false
else
  echo "Building a release version"
  COMMIT=$(git rev-parse HEAD)
  RELEASE=true
fi
echo $COMMIT

echo -n "Tagged version: "
if git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*"; then
  VERSION=$(git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*")
  echo $VERSION
else
  VERSION=$(git show -s --date='format:%Y.%m' --format='v%cd.%ct-%h' HEAD)
  RELEASE=false
fi

echo Running "go $@"
exec go "$1" -ldflags \
  "-X storj.io/common/version.buildTimestamp=$TIMESTAMP
   -X storj.io/common/version.buildCommitHash=$COMMIT
   -X storj.io/common/version.buildVersion=$VERSION
   -X storj.io/common/version.buildRelease=$RELEASE" "${@:2}"
