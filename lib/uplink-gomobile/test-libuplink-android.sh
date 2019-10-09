#!/bin/bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

cd "$SCRIPTDIR/libuplink_android"

# Might be easier way than -Pandroid.testInstrumentationRunnerArguments
./gradlew connectedAndroidTest -Pandroid.testInstrumentationRunnerArguments.scope=$GATEWAY_0_SCOPE
./gradlew clean