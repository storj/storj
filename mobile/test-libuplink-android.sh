#!/bin/bash
set -ueo pipefail

# TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

# cleanup(){
#     rm -rf "$TMPDIR"
#     echo "cleaned up test successfully"
# }

# trap cleanup EXIT

echo "Executing gomobile bind"

gomobile bind -target android -o libuplink_android/app/libs/libuplink.aar -javapkg io.storj.libuplink storj.io/storj/mobile

cd libuplink_android

# Might be easier way than -Pandroid.testInstrumentationRunnerArguments
./gradlew connectedAndroidTest -Pandroid.testInstrumentationRunnerArguments.api.key=$GATEWAY_0_API_KEY