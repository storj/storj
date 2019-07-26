#!/usr/bin/env bash
set -ueo pipefail

SRC_DIR=$1
BUCKET=$2


UPLINK_DEBUG_ADDR=""

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" mb "sj://$BUCKET/"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/big-upload-testfile" "sj://$BUCKET/"