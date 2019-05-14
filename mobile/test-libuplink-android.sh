#!/bin/bash
set -ueo pipefail

echo "Executing gomobile bind"

gomobile bind -target android -o libuplink_android/app/libs/libuplink-android.aar -javapkg io.storj.libuplink storj.io/storj/mobile

cd libuplink_android

# Might be easier way than -Pandroid.testInstrumentationRunnerArguments
./gradlew connectedAndroidTest -Pandroid.testInstrumentationRunnerArguments.api.key=$GATEWAY_0_API_KEY -Pandroid.testInstrumentationRunnerArguments.storj.sim.host=$SATELLITE_0_ADDR