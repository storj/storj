#!/bin/bash
set -ueo pipefail

cd "$TEST_PROJECT"

# Might be easier way than -Pandroid.testInstrumentationRunnerArguments
./gradlew connectedAndroidTest -Pandroid.testInstrumentationRunnerArguments.scope=$GATEWAY_0_SCOPE