#!/bin/bash

# This script will build libuplink-android.aar library from scratch
# Required:
# * ANDROID_HOME set with NDK available
# * go

if [ -z "$ANDROID_HOME" ]
then
      echo "\$ANDROID_HOME is not set" && exit 1
fi

if [ ! -d "$ANDROID_HOME/ndk-bundle" ]
then
      echo "ANDROID NDK is not available" && exit 1
fi

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
GOPATH_SYSTEM=$(go env GOPATH)
cleanup(){
      if [ -z "$GOPATH_SYSTEM" ]
      then
            # cleanup pkg/mod directory
            go clean -modcache
      fi
	rm -rf "$TMP"
}
trap cleanup EXIT

OUTPUT=$PWD

mkdir -p "$TMP/pkg/mod/" "$TMP/src/storj.io/storj/"
if [ -n "$GOPATH_SYSTEM" ]
then
      # use this only if GOPATH is set
      # link pkg/mod to avoid redownloading modules
      ln -s $GOPATH_SYSTEM/pkg/mod/* "$TMP/pkg/mod/"
fi

# go knows where our gopath is
export GOPATH=$TMP

# symlink doesn't look to be working with gomobile
rsync -am --stats --exclude=".*" $SCRIPTDIR/../../* "$GOPATH/src/storj.io/storj/"

cd "$GOPATH/src/storj.io/storj"

go mod vendor

cp -r $GOPATH/src/storj.io/storj/vendor/* "$GOPATH/src"

# set go modules to default behavior
export GO111MODULE=off

go get golang.org/x/mobile/cmd/gomobile

# add gobind to PATH
export PATH=$GOPATH/bin:$PATH

$GOPATH/bin/gomobile init

$GOPATH/bin/gomobile bind -v -target android -o "$OUTPUT/libuplink-android.aar" -javapkg io.storj.libuplink storj.io/storj/lib/uplink-gomobile