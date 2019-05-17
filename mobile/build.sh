#!/bin/bash

# This script will build libuplink-android.aar library from scratch
# Required:
# * ANDROID_HOME set with NDK available
# * go
# * gospace

if [ -z "$ANDROID_HOME" ]
then
      echo "\$ANDROID_HOME is not set"
      exit 1
fi

OUTPUT_DIR=${1:-$PWD}
OUTPUT_AAR="libuplink-android.aar"
OUTPUT_JAVA_PACKAGE="io.storj.libuplink"

STORJ_PATH=~/storj-for-android

# set go modules to default behavior
export GO111MODULE=auto

# go knows where our gopath is
export GOPATH=$STORJ_PATH

# gospace knows where our gopath is (this is to avoid accidental damage to existing GOPATH)
# you should not use default GOPATH here
export GOSPACE_ROOT=$STORJ_PATH

# set the github repository that this GOSPACE manages
export GOSPACE_PKG=storj.io/storj

# set the where the repository is located
export GOSPACE_REPO=git@github.com:storj/storj.git

# TODO add check if STORJ_PATH exists do only update
gospace setup

export PATH=$PATH:$GOPATH/bin

# step can be removed after merge to master
cd $GOPATH/src/storj.io/storj
git checkout -q mn/java-bindings

cd $GOPATH

go get golang.org/x/mobile/cmd/gomobile

gomobile init

gomobile bind -v -target android -o $OUTPUT_DIR/libuplink-android.aar -javapkg $OUTPUT_JAVA_PACKAGE storj.io/storj/mobile
