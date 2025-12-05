#!/usr/bin/env bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source $SCRIPTDIR/../../scripts/utils.sh

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT
trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

: "${STORJ_NETWORK_DIR?Environment variable STORJ_NETWORK_DIR needs to be set}"

while getopts "b:" o; do
    case "${o}" in
    b)
        BUCKET="${OPTARG}"
        ;;
    *)
        ;;
    esac
done
shift $((OPTIND-1))

BUCKET="${BUCKET:-bucket-123}"
PRISTINE_FILES_DIR="$STORJ_NETWORK_DIR/pristine/$BUCKET"
DOWNLOAD_FILES_DIR="$STORJ_NETWORK_DIR/download/$BUCKET"

if [[ ! -v UPLINK_ACCESS ]]; then
  # override configured access with access where address is node ID + satellite addess
  STORJ_ACCESS=$(go run "$SCRIPTDIR"/../../testsuite/update-access.go "$SATELLITE_0_DIR" "$GATEWAY_0_ACCESS")
  UPLINK_ACCESS="$STORJ_ACCESS"

  export STORJ_ACCESS
  export UPLINK_ACCESS
fi

# workaround for issues with automatic accepting monitoring question
# with first run we need to accept question y/n about monitoring
export UPLINK_CONFIG_DIR=$TMPDIR/uplink
mkdir -p "$UPLINK_CONFIG_DIR"
touch "$UPLINK_CONFIG_DIR/config.ini"

set -x

if [[ "$1" == "upload" ]]; then
    mkdir -p "$PRISTINE_FILES_DIR" "$DOWNLOAD_FILES_DIR"

    random_bytes_file "2048"   "$PRISTINE_FILES_DIR/small-upload-testfile"         # create 2kb file of random bytes (inline)
    random_bytes_file "5242880"   "$PRISTINE_FILES_DIR/big-upload-testfile"           # create 5mb file of random bytes (remote)
    random_bytes_file "12582912"  "$PRISTINE_FILES_DIR/multisegment-upload-testfile"  # create 12mb file of random bytes (remote)

    # sometimes we overwrite files in the same bucket. allow the mb to fail because of an existing
    # bucket. if it fails for any other reason, the following cp will get it anyway.
    uplink mb "sj://$BUCKET/" || true

    uplink cp --progress=false "$PRISTINE_FILES_DIR/small-upload-testfile" "sj://$BUCKET/"
    uplink cp --progress=false "$PRISTINE_FILES_DIR/big-upload-testfile" "sj://$BUCKET/"
    uplink cp --progress=false "$PRISTINE_FILES_DIR/multisegment-upload-testfile" "sj://$BUCKET/"
fi

if [[ "$1" == "download" ]]; then
    uplink cp --progress=false "sj://$BUCKET/small-upload-testfile" "$DOWNLOAD_FILES_DIR"
    uplink cp --progress=false "sj://$BUCKET/big-upload-testfile" "$DOWNLOAD_FILES_DIR"
    uplink cp --progress=false "sj://$BUCKET/multisegment-upload-testfile" "$DOWNLOAD_FILES_DIR"

    compare_files "$PRISTINE_FILES_DIR/small-upload-testfile" "$DOWNLOAD_FILES_DIR/small-upload-testfile"
    compare_files "$PRISTINE_FILES_DIR/big-upload-testfile" "$DOWNLOAD_FILES_DIR/big-upload-testfile"
    compare_files "$PRISTINE_FILES_DIR/multisegment-upload-testfile" "$DOWNLOAD_FILES_DIR/multisegment-upload-testfile"

    rm "$DOWNLOAD_FILES_DIR/small-upload-testfile"
    rm "$DOWNLOAD_FILES_DIR/big-upload-testfile"
    rm "$DOWNLOAD_FILES_DIR/multisegment-upload-testfile"
fi

if [[ "$1" == "cleanup" ]]; then
    for BUCKET_DIR in "$STORJ_NETWORK_DIR"/pristine/*; do
        BUCKET="$(basename "$BUCKET_DIR")"
        uplink rm "sj://$BUCKET/small-upload-testfile"
        uplink rm "sj://$BUCKET/big-upload-testfile"
        uplink rm "sj://$BUCKET/multisegment-upload-testfile"
        uplink rb "sj://$BUCKET"
    done
fi
