#!/usr/bin/env bash

set -ueo pipefail

main_cfg_dir=$1
update_access_script_path=$2

bucket="old-satellite"
test_dir="${main_cfg_dir}/test"
test_src_dir="${test_dir}/src"
test_dst_dir="${test_dir}/dst"
test_upl_dir="${test_dir}/uplink"

echo "Begin rolling-upgrade/test-previous-satellite.sh, storj-sim config directory: ${main_cfg_dir}/local-network"

export PATH="${main_cfg_dir}/bin:$PATH"

echo "Which storj-sim: $(which storj-sim)"
echo "Shasum for storj-sim:"
shasum $(which storj-sim)

echo -e "\nConfig directory for uplink: ${main_cfg_dir}/uplink"
echo "Which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum $(which uplink)

access=$(storj-sim --config-dir="$main_cfg_dir/local-network" network env GATEWAY_0_ACCESS)
new_access=$(go run "${update_access_script_path}" -a "127.0.0.1:30000" $(storj-sim --config-dir=$main_cfg_dir/local-network network env SATELLITE_0_DIR) "${access}")
echo "Access: ${new_access}"

mkdir -p "${test_src_dir}" "${test_dst_dir}" "${test_upl_dir}"
random_bytes_file () {
    size=$1
    output=$2
    head -c $size </dev/urandom > $output
}
random_bytes_file "2KiB"  "$test_src_dir/small-upload-testfile"

echo "Setup uplink"
echo "access: ${new_access}" > "${test_upl_dir}/config.yaml"
echo -e "[analytics]\nenabled = false\n\n[metrics]\naddr =" > "${test_upl_dir}/config.ini"
uplink --config-dir="${test_upl_dir}" access import default "${new_access}"

echo "Creating bucket sj://$bucket/"
uplink mb --config-dir="${test_upl_dir}" "sj://$bucket/"

echo "Uploading ${test_src_dir}/small-upload-testfile to sj://$bucket/"
uplink cp --config-dir="${test_upl_dir}" --progress=false "${test_src_dir}/small-upload-testfile" "sj://$bucket/"

echo "Downloading sj://$bucket/small-upload-testfile to ${test_dst_dir}"
uplink cp --config-dir="${test_upl_dir}" --progress=false "sj://$bucket/small-upload-testfile" "${test_dst_dir}"

if cmp "${test_src_dir}/small-upload-testfile" "${test_dst_dir}/small-upload-testfile"
then
    echo "Upload test on release tag: small upload testfile matches uploaded file"
else
    echo "Upload test on release tag: small upload testfile does not match uploaded file"
    exit 1
fi

echo "Deleting object sj://$bucket/small-upload-testfile"
uplink rm --config-dir="${test_upl_dir}" "sj://$bucket/small-upload-testfile"

echo "Deleting bucket sj://$bucket"
uplink rb --config-dir="${test_upl_dir}" "sj://$bucket"

rm -rf "${test_src_dir}"
rm -rf "${test_dst_dir}"
rm -rf "${test_upl_dir}"

echo "Done with rolling-upgrade/test-previous-satellite.sh"
