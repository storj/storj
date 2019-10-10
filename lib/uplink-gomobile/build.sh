#!/bin/bash

# This script will build libuplink-android.aar library from scratch
# Required:
# * ANDROID_HOME set with NDK available
# * go

if [ -z "$ANDROID_HOME" ]
then
      echo "\$ANDROID_HOME is not set"
      exit 1
fi

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

OUTPUT=$PWD

# go knows where our gopath is
export GOPATH=$TMP

mkdir -p "$GOPATH/src/storj.io/storj/"

# symlink doesn't look to be working with gomobile
rsync -am --stats --exclude=".*" ./* "$GOPATH/src/storj.io/storj/"

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

# cleanup pkg/mod directory
go clean -modcache