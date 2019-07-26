#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# bash script arguments
OLD_REL_NAME=$1
NEW_REL_NAME=$2
BUCKET=bucket-123

# setup tmpdir for releases to test 
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

SRC_DIR=$TMP/source
DST_DIR=$TMP/dst

mkdir -p "$SRC_DIR" "$DST_DIR" 

random_bytes_file () {
    size=$1
    output=$2
    dd if=/dev/urandom of="$output" count=1 bs="$size" >/dev/null 2>&1
}

random_bytes_file 2x1024      "$SRC_DIR/small-upload-testfile" # create 2kb file of random bytes (inline)
random_bytes_file 5x1024x1024 "$SRC_DIR/big-upload-testfile"   # create 5mb file of random bytes (remote)

OLD_REL_DIR=$TMP/old
NEW_REL_DIR=$TMP/new
mkdir -p "$OLD_REL_DIR" "$NEW_REL_DIR"

# $1->download dir ; $2->release name
function download_build_release {
    cd $1
    git clone https://github.com/storj/storj -b $2
    cd $1/storj
    make install-sim
}

cleanup() {
    echo "cleaning .... "
    if [ "$TMP" ]; then
	    rm -rf "$TMP"
    fi
}

trap cleanup EXIT

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# download and build new release
download_build_release $NEW_REL_DIR $NEW_REL_NAME

# run uplink upload tests
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-upload.sh $SRC_DIR $BUCKET 

# download and build old release
download_build_release $OLD_REL_DIR $OLD_REL_NAME

# run uplink downlaod tests
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-download.sh $BUCKET $DST_DIR
storj-sim -x --satellites 2 --host $STORJ_NETWORK_HOST4 network destroy

if cmp "$SRC_DIR/small-upload-testfile" "$DST_DIR/small-upload-testfile"
then
    echo "small upload testfile matches uploaded file"
else
    echo "small upload testfile does not match uploaded file"
    exit 1
fi

if cmp "$SRC_DIR/big-upload-testfile" "$DST_DIR/big-upload-testfile"
then
    echo "big upload testfile matches uploaded file"
else
    echo "big upload testfile does not match uploaded file"
    exit 1
fi