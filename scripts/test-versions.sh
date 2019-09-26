#!/usr/bin/env bash
set -ueo pipefail

main_cfg_dir=$1

echo "Begin test-versions.sh" ${main_cfg_dir}

echo "which storj-sim"
which storj-sim
shasum $(which storj-sim)

echo "Uplink version:"
uplink version --config-dir "${main_cfg_dir}/gateway/0/"

echo "Satellite config directory:"
storj-sim network env --config-dir "${main_cfg_dir}" SATELLITE_0_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" SATELLITE_0_DIR)/satellite

echo "Storagenode config directories:"
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_0_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_0_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_1_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_1_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_2_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_2_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_3_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_3_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_4_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_4_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_5_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_5_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_6_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_6_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_7_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_7_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_8_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_8_DIR)/storagenode
storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_9_DIR
shasum $(storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_9_DIR)/storagenode

echo "!!!!!!!!!!!!"
