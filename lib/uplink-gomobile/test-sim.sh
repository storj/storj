#!/bin/bash
set -ueo pipefail
set +x

if [ -z "$ANDROID_HOME" ]
then
      echo "\$ANDROID_HOME is not set"
      exit 1
fi

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

# start Android emulator
PORT=6000
SERIAL=emulator-${PORT}
AVD_NAME=uplink_test

export PATH=$ANDROID_HOME/emulator/:$PATH

echo "no" | $ANDROID_HOME/tools/bin/avdmanager create avd --name "${AVD_NAME}" -k "system-images;android-24;default;x86_64" --force
echo "AVD ${AVD_NAME} created."

$ANDROID_HOME/emulator/emulator -avd ${AVD_NAME} -port ${PORT} -no-window -no-accel -no-audio -no-boot-anim 2>&1 &

# copy test project and build aar file
cp -r "$SCRIPTDIR/libuplink_android/" "$TMP/libuplink_android"
mkdir -p "$TMP/libuplink_android/app/libs/"
cd "$TMP/libuplink_android/app/libs/" && $SCRIPTDIR/build.sh
export TEST_PROJECT="$TMP/libuplink_android/"

#Ensure Android Emulator has booted successfully before continuing
# TODO add max number of checks and timeout
while [ "`adb shell getprop sys.boot_completed | tr -d '\r' `" != "1" ] ; do sleep 3; done

# start integration tests
export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}

# setup the network
storj-sim -x --host $STORJ_NETWORK_HOST4 network setup

# run tests
storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR/test-libuplink-android.sh"
storj-sim -x --host $STORJ_NETWORK_HOST4 network destroy

$ANDROID_HOME/platform-tools/adb -s ${SERIAL} emu kill