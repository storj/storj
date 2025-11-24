#!/usr/bin/env bash

## This script:
#
# 1) Makes sure the current git working tree is clean
# 2) Creates the git tag
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
if git tag $VERSION; then
  echo successfully created tag $VERSION
fi