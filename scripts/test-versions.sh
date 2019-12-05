#!/usr/bin/env bash
set -ueo pipefail

main_cfg_dir=$1
command=$2

bucket="bucket-123"
test_files_dir="${main_cfg_dir}/testfiles"
stage1_dst_dir="${main_cfg_dir}/stage1"
stage2_dst_dir="${main_cfg_dir}/stage2"

setup(){
    mkdir -p "$test_files_dir" "$stage1_dst_dir" "$stage2_dst_dir"
    random_bytes_file () {
        size=$1
        output=$2
	    head -c $size </dev/urandom > $output
    }
    random_bytes_file "2048"   "$test_files_dir/small-upload-testfile"          # create 2kb file of random bytes (inline)
    random_bytes_file "5242880" "$test_files_dir/big-upload-testfile"            # create 5mb file of random bytes (remote)
    random_bytes_file "134217728" "$test_files_dir/multisegment-upload-testfile"   # create 128mb file of random bytes (remote)

    echo "setup test successfully"
}

echo "Begin test-versions.sh, storj-sim config directory:" ${main_cfg_dir}

echo "which storj-sim: $(which storj-sim)"
# shasum $(which storj-sim)

echo -e "\nConfig directory for uplink:"
echo "${main_cfg_dir}/gateway/0"
echo "which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum $(which uplink)
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

if [[ "$command" == "upload" ]]; then
    uplink_version=$3
    setup
    bucket_name=${bucket}-${uplink_version}
    download_dst_dir=${stage1_dst_dir}/${uplink_version}
    mkdir -p "$download_dst_dir"

    uplink --config-dir "${main_cfg_dir}/gateway/0" mb "sj://$bucket_name/"

    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "${test_files_dir}/small-upload-testfile" "sj://$bucket_name/"
    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "${test_files_dir}/big-upload-testfile" "sj://$bucket_name/"
    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "${test_files_dir}/multisegment-upload-testfile" "sj://$bucket_name/"

    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}"
    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}"
    uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}"

    if cmp "${test_files_dir}/small-upload-testfile" "${download_dst_dir}/small-upload-testfile"
    then
        echo "upload test on release tag: small upload testfile matches uploaded file"
    else
        echo "upload test on release tag: small upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "${test_files_dir}/big-upload-testfile" "${download_dst_dir}/big-upload-testfile"
    then
        echo "upload test on release tag: big upload testfile matches uploaded file"
    else
        echo "upload test on release tag: big upload testfile does not match uploaded file"
        exit 1
    fi

    if cmp "${test_files_dir}/multisegment-upload-testfile" "${download_dst_dir}/multisegment-upload-testfile"
    then
        echo "upload test on release tag: multisegment upload testfile matches uploaded file"
    else
        echo "upload test on release tag: multisegment upload testfile does not match uploaded file"
        exit 1
    fi

fi

if [[ "$command" == "download" ]]; then
    setup
    existing_bucket_name_suffixes=$3

    for suffix in ${existing_bucket_name_suffixes}; do
        bucket_name=${bucket}-${suffix}
        original_dst_dir=${stage1_dst_dir}/${suffix}
        download_dst_dir=${stage2_dst_dir}/${suffix}
        mkdir -p "$download_dst_dir"

        echo "bucket name: ${bucket_name}"
        echo "download folder name: ${download_dst_dir}"
        uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}"
        uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}"
         uplink --config-dir "${main_cfg_dir}/gateway/0" cp --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}"

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
    done

    # rm "${stage2_dst_dir}/small-upload-testfile"
    # rm "${stage2_dst_dir}/big-upload-testfile"
    # rm "${stage2_dst_dir}/multisegment-upload-testfile"
fi

if [[ "$command" == "cleanup" ]]; then
    uplink_versions=$3
    setup
    for ul_version in ${uplink_versions}; do
        bucket_name=${bucket}-${ul_version}
        uplink --config-dir "${main_cfg_dir}/gateway/0" rm "sj://$bucket_name/small-upload-testfile"
        uplink --config-dir "${main_cfg_dir}/gateway/0" rm "sj://$bucket_name/big-upload-testfile"
        uplink --config-dir "${main_cfg_dir}/gateway/0" rm "sj://$bucket_name/multisegment-upload-testfile"
        uplink --config-dir "${main_cfg_dir}/gateway/0" rb "sj://$bucket_name"
    done
fi

echo "Done with test-versions.sh"
