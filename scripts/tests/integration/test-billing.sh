#!/usr/bin/env bash
set -ueo pipefail

PERIOD=$(date -d "$(date +%Y-%m)-15 last month" '+%Y-%m')
satellite --config-dir $SATELLITE_0_DIR billing prepare-invoice-records $PERIOD
satellite --config-dir $SATELLITE_0_DIR billing create-invoice-items    $PERIOD
satellite --config-dir $SATELLITE_0_DIR billing create-invoice-coupons  $PERIOD
satellite --config-dir $SATELLITE_0_DIR billing create-invoices         $PERIOD
satellite --config-dir $SATELLITE_0_DIR billing finalize-invoices       $PERIOD