#!/usr/bin/env bash

 set -eu
set -o pipefail

 echo -n "Build timestamp: "
TIMESTAMP=$(date +%s)
echo $TIMESTAMP

 echo -n "Git commit: "
if [[ "$(git diff --stat)" != '' ]] || [[ -n "$(git status -s)" ]]; then
  COMMIT=$(git rev-parse HEAD)-dirty
  RELEASE=false
else
  COMMIT=$(git rev-parse HEAD)
  RELEASE=true
fi
echo $COMMIT

 echo -n "Tagged version: "
VERSION=$(git describe --tags --match "v[0-9]*.[0-9]*.[0-9]*")
echo $VERSION

 echo Running "go $@"
exec go "$1" -ldflags \
	"-X storj.io/storj/internal/version.Timestamp=$TIMESTAMP
         -X storj.io/storj/internal/version.CommitHash=$COMMIT
         -X storj.io/storj/internal/version.Version=$VERSION
         -X storj.io/storj/internal/version.Release=$RELEASE" "${@:2}"
