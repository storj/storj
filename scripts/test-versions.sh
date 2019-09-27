#!/usr/bin/env bash
set -ueo pipefail

main_cfg_dir=$1

echo "Begin test-versions.sh, storj-sim config directory:" ${main_cfg_dir}

echo "which storj-sim: $(which storj-sim)"
# shasum $(which storj-sim)

echo -e "\nConfig directory for uplink:"
echo "${main_cfg_dir}/gateway/0"
echo "which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum uplink
# uplink version --config-dir "${main_cfg_dir}/gateway/0/"

echo -e "\nConfig directory for satellite:"
echo "${main_cfg_dir}/satellite/0"
# storj-sim network env --config-dir "${main_cfg_dir}" SATELLITE_0_DIR
echo "Shasum for satellite:"
shasum ${main_cfg_dir}/satellite/0/satellite

echo -e "\nStoragenode config directories:"
for i in {0..9}
do 
    echo -e "\nConfig directory for sn ${i}:"
    echo "${main_cfg_dir}/storagenode/${i}"
    # storj-sim network env --config-dir "${main_cfg_dir}" STORAGENODE_${i}_DIR
    echo "Shasum for sn ${i} binary:"
    shasum ${main_cfg_dir}/storagenode/${i}/storagenode
done

echo "Done with test-versions.sh"
