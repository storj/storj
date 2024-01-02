#!/usr/bin/env bash

# This file contains the first part of Stage 2 for the rolling upgrade test.
# Description of file functionality:
#  * Download the inline, remote, and multisegment file from the network using the main uplink and the new satellite api.
#  * Download the inline, remote, and multisegment file from the network using the main uplink and the old satellite api.

set -ueo pipefail

# This script assumes that storj-sim and uplink has already been set up and initial files have been
# uploaded via /uplink-versions/steps.sh
main_cfg_dir=$1
existing_bucket_name_suffixes=$2
update_access_script_path=$3

bucket="bucket-123"
test_files_dir="${main_cfg_dir}/testfiles"
stage1_dst_dir="${main_cfg_dir}/stage1"
stage2_dst_dir="${main_cfg_dir}/stage2"

echo "Begin rolling-upgrade/step-1.sh, storj-sim config directory:" ${main_cfg_dir}

echo "which storj-sim: $(which storj-sim)"
echo "Shasum for storj-sim:"
shasum $(which storj-sim)

if [ ! -d ${main_cfg_dir}/uplink-old-api ]; then
    mkdir -p ${main_cfg_dir}/uplink-old-api
    access=$(storj-sim --config-dir=$main_cfg_dir network env GATEWAY_0_ACCESS)
    new_access=$(go run $update_access_script_path $(storj-sim --config-dir=$main_cfg_dir network env SATELLITE_0_DIR) $access)
    echo "access: ${new_access}" > "${main_cfg_dir}/uplink-old-api/config.yaml"
fi

echo -e "\nConfig directory for uplink:"
echo "${main_cfg_dir}/uplink"
echo "which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum $(which uplink)

new_uplink() {
    UPLINK_LEGACY_CONFIG_DIR="${main_cfg_dir}/uplink" uplink --config-dir="${main_cfg_dir}/uplink" "$@"
}

old_uplink() {
    UPLINK_LEGACY_CONFIG_DIR="${main_cfg_dir}/uplink-old-api" uplink --config-dir="${main_cfg_dir}/uplink-old-api" "$@"
}

echo -e "\nConfig directory for satellite:"
echo "${main_cfg_dir}/satellite/0"
echo "Shasum for satellite:"
shasum ${main_cfg_dir}/satellite/0/satellite

echo "Shasum for old satellite:"
shasum ${main_cfg_dir}/satellite/0/old_satellite

echo -e "\nStoragenode config directories:"
for i in {0..9}
do
    echo -e "\nConfig directory for sn ${i}:"
    echo "${main_cfg_dir}/storagenode/${i}"
    echo "Shasum for sn ${i} binary:"
    shasum ${main_cfg_dir}/storagenode/${i}/storagenode
done

for suffix in ${existing_bucket_name_suffixes}; do
    bucket_name=${bucket}-${suffix}
    original_dst_dir=${stage1_dst_dir}/${suffix}
    download_dst_dir=${stage2_dst_dir}/${suffix}
    old_api_download_dst_dir=${download_dst_dir}/old-api
    mkdir -p "$download_dst_dir"
    mkdir -p "$old_api_download_dst_dir"

    echo "bucket name: ${bucket_name}"
    echo "download folder name: ${download_dst_dir}"

    new_uplink cp --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}"
    new_uplink cp --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}"
    new_uplink cp --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}"

    old_uplink cp --progress=false "sj://$bucket_name/small-upload-testfile" "${old_api_download_dst_dir}"
    old_uplink cp --progress=false "sj://$bucket_name/big-upload-testfile" "${old_api_download_dst_dir}"
    old_uplink cp --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${old_api_download_dst_dir}"

    if cmp "${original_dst_dir}/small-upload-testfile" "${download_dst_dir}/small-upload-testfile"
    then
        echo "download test on current branch: small upload testfile matches uploaded file"
    else
        echo "download test on current branch: small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "${original_dst_dir}/big-upload-testfile" "${download_dst_dir}/big-upload-testfile"
    then
        echo "download test on current branch: big upload testfile matches uploaded file"
    else
        echo "download test on current branch: big upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "${original_dst_dir}/multisegment-upload-testfile" "${download_dst_dir}/multisegment-upload-testfile"
    then
        echo "download test on current branch: multisegment upload testfile matches uploaded file"
    else
        echo "download test on current branch: multisegment upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "${original_dst_dir}/small-upload-testfile" "${old_api_download_dst_dir}/small-upload-testfile"
    then
        echo "download test on current branch: small upload testfile (old api) matches uploaded file"
    else
        echo "download test on current branch: small upload testfile (old api) does not match uploaded file"
        exit 1
    fi

    if cmp "${original_dst_dir}/big-upload-testfile" "${old_api_download_dst_dir}/big-upload-testfile"
    then
        echo "download test on current branch: big upload testfile (old api) matches uploaded file"
    else
        echo "download test on current branch: big upload testfile (old api) does not match uploaded file"
        exit 1
    fi

    if cmp "${original_dst_dir}/multisegment-upload-testfile" "${old_api_download_dst_dir}/multisegment-upload-testfile"
    then
        echo "download test on current branch: multisegment upload testfile (old api) matches uploaded file"
    else
        echo "download test on current branch: multisegment upload testfile (old api) does not match uploaded file"
        exit 1
    fi

    rm -rf ${download_dst_dir}
done

echo "Done with rolling-upgrade/step-1.sh"
