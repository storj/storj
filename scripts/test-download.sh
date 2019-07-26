#!/usr/bin/env bash
set -ueo pipefail

BUCKET=$1
DST_DIR=$2

UPLINK_DEBUG_ADDR=""

# uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/small-upload-testfile" "sj://$BUCKET/"
# uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "$SRC_DIR/big-upload-testfile" "sj://$BUCKET/"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "sj://$BUCKET/small-upload-testfile" "$DST_DIR"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" cp "sj://$BUCKET/big-upload-testfile" "$DST_DIR"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rm "sj://$BUCKET/small-upload-testfile"
uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rm "sj://$BUCKET/big-upload-testfile"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" ls "sj://$BUCKET"

uplink --config-dir "$GATEWAY_0_DIR" --debug.addr "$UPLINK_DEBUG_ADDR" rb "sj://$BUCKET"
