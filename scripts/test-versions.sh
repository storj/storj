#!/usr/bin/env bash
set -ueo pipefail

ul_cfg_dir=$1

echo "Uplink version:"
uplink version --config-dir $ul_cfg_dir