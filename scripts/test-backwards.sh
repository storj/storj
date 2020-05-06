#!/usr/bin/env bash
set -ueo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

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

# override configured access with access where address is node ID + satellite addess
STORJ_ACCESS=$(go run "$SCRIPTDIR"/update-access.go "$SATELLITE_0_DIR" "$GATEWAY_0_ACCESS")
export STORJ_ACCESS

set -x

if [[ "$1" == "upload" ]]; then
    mkdir -p "$PRISTINE_FILES_DIR" "$DOWNLOAD_FILES_DIR"

    random_bytes_file () {
        size=$1
        output=$2
        head -c "$size" </dev/urandom > "$output"
    }
    random_bytes_file "2KiB"   "$PRISTINE_FILES_DIR/small-upload-testfile"         # create 2kb file of random bytes (inline)
    random_bytes_file "5MiB"   "$PRISTINE_FILES_DIR/big-upload-testfile"           # create 5mb file of random bytes (remote)
    random_bytes_file "65MiB"  "$PRISTINE_FILES_DIR/multisegment-upload-testfile"  # create 65mb file of random bytes (remote)

    # sometimes we overwrite files in the same bucket. allow the mb to fail because of an existing
    # bucket. if it fails for any other reason, the following cp will get it anyway.
    uplink --config-dir "$GATEWAY_0_DIR" mb "sj://$BUCKET/" || true

    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "$PRISTINE_FILES_DIR/small-upload-testfile" "sj://$BUCKET/"
    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "$PRISTINE_FILES_DIR/big-upload-testfile" "sj://$BUCKET/"
    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "$PRISTINE_FILES_DIR/multisegment-upload-testfile" "sj://$BUCKET/"
fi

if [[ "$1" == "download" ]]; then
    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "sj://$BUCKET/small-upload-testfile" "$DOWNLOAD_FILES_DIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "sj://$BUCKET/big-upload-testfile" "$DOWNLOAD_FILES_DIR"
    uplink --config-dir "$GATEWAY_0_DIR" cp --progress=false "sj://$BUCKET/multisegment-upload-testfile" "$DOWNLOAD_FILES_DIR"

    if cmp "$PRISTINE_FILES_DIR/small-upload-testfile" "$DOWNLOAD_FILES_DIR/small-upload-testfile"
    then
        echo "download test: small upload testfile matches uploaded file"
    else
        echo "download test: small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$PRISTINE_FILES_DIR/big-upload-testfile" "$DOWNLOAD_FILES_DIR/big-upload-testfile"
    then
        echo "download test: big upload testfile matches uploaded file"
    else
        echo "download test: big upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "$PRISTINE_FILES_DIR/multisegment-upload-testfile" "$DOWNLOAD_FILES_DIR/multisegment-upload-testfile"
    then
        echo "download test: multisegment upload testfile matches uploaded file"
    else
        echo "download test: multisegment upload testfile does not match uploaded file"
        exit 1
    fi

    rm "$DOWNLOAD_FILES_DIR/small-upload-testfile"
    rm "$DOWNLOAD_FILES_DIR/big-upload-testfile"
    rm "$DOWNLOAD_FILES_DIR/multisegment-upload-testfile"
fi

if [[ "$1" == "cleanup" ]]; then
    for BUCKET_DIR in "$STORJ_NETWORK_DIR"/pristine/*; do
        BUCKET="$(basename "$BUCKET_DIR")"
        uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/small-upload-testfile"
        uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/big-upload-testfile"
        uplink --config-dir "$GATEWAY_0_DIR" rm "sj://$BUCKET/multisegment-upload-testfile"
        uplink --config-dir "$GATEWAY_0_DIR" rb "sj://$BUCKET"
    done
fi
