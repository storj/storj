#!/usr/bin/env bash
# We need to identify the version based on the used directories.
# Any change in the version will trigger new build for docker layer, therefore we don't need new version
# if nothing has been changed.
# You can defined the subset of directories with MODULE=storagenode ./bake.sh ....
SATELLITE_DIRS="./satellite ./shared ./private ./go.mod ./go.sum"
STORAGENODE_DIRS="./storagenode ./shared ./private ./go.mod ./go.sum ./web/storagenode"

# Set DIRS based on MODULE
if [ -z "$MODULE" ]; then
  DIRS="."
else
  varname="${MODULE}_DIRS"
  DIRS=${!varname}
fi

echo "Identifying versions from dir(s): $DIRS"

export BUILD_COMMIT=$(git log -1 --pretty=format:'%H' -- $DIRS)
echo "Build commit: $BUILD_COMMIT"

export BUILD_DATE="$(git log -1 $BUILD_COMMIT --pretty=format:'%ct' -- $DIRS)"
echo "Build timestamp: $BUILD_DATE"

# determine version (either from Git or explicitly specified)
if BUILD_VERSION=$(git describe --tags --exact-match --match "v[0-9]*.[0-9]*.[0-9]*" 2>/dev/null); then
  echo "Using tagged version: $BUILD_VERSION"
else
  BUILD_VERSION=$(git log -1 --date='format:%Y.%m' --format='v%cd.%ct-%h' HEAD -- $DIRS)
  echo "Using commit-based version: $BUILD_VERSION"
fi
export BUILD_VERSION

docker buildx bake $@
