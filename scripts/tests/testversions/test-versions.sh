#!/usr/bin/env bash

set -ueo pipefail

main_cfg_dir=$1
command=$2
uplink_version=$3
update_access_script_path=$4

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
    random_bytes_file "2KiB"  "$test_files_dir/small-upload-testfile"         # create 2kb file of random bytes (inline)
    random_bytes_file "5KiB"  "$test_files_dir/big-upload-testfile"           # create 5kb file of random bytes (remote)
    random_bytes_file "64MiB" "$test_files_dir/multisegment-upload-testfile"  # create 64mb file of random bytes (remote + inline)

    echo "setup test successfully"
}

wait_for_all_background_jobs_to_finish(){
    for job in `jobs -p`
    do
        echo "wait for $job"
        RESULT=0
        wait $job || RESULT=1
        if [ "$RESULT" == "1" ]; then
            echo "job $job failed"
        fi
    done
}

echo "Begin test-versions.sh, storj-sim config directory:" ${main_cfg_dir}

echo "which storj-sim: $(which storj-sim)"
echo "Shasum for storj-sim:"
shasum $(which storj-sim)

echo -e "\nConfig directory for uplink:"
echo "${main_cfg_dir}/uplink"
echo "which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum $(which uplink)

# for oldest uplink versions, access is not supported, and we need to configure separate values for api key, sat addr, and encryption key
if [ ! -d ${main_cfg_dir}/uplink ]; then
    mkdir -p ${main_cfg_dir}/uplink
    api_key=$(storj-sim --config-dir=$main_cfg_dir network env GATEWAY_0_API_KEY)
    sat_addr=$(storj-sim --config-dir=$main_cfg_dir network env SATELLITE_0_ADDR)
    should_use_access=$(echo $uplink_version | awk 'BEGIN{FS="[v.]"} $3 >= 30 || $2 >= 1 {print $0}')
    if [[ ${#should_use_access} -gt 0 ]]; then
        access=$(storj-sim --config-dir=$main_cfg_dir network env GATEWAY_0_ACCESS)
        new_access=$(go run $update_access_script_path $(storj-sim --config-dir=$main_cfg_dir network env SATELLITE_0_DIR) $access)
        uplink import --config-dir="${main_cfg_dir}/uplink" "${new_access}"
    else
        uplink setup --config-dir="${main_cfg_dir}/uplink" --non-interactive --api-key="$api_key" --satellite-addr="$sat_addr" --enc.encryption-key="test"
    fi
fi

if [[ $uplink_version = "v0.29.10" ]]; then
    uplink share --config-dir="${main_cfg_dir}/uplink" | grep "Scope" | awk -F ": " '{print $2}' | tee ${main_cfg_dir}/uplink/access.txt
fi

# after version v0.30.x we need to use access instead of separate values for api key, sat addr, and encryption key
should_use_access=$(echo $uplink_version | awk 'BEGIN{FS="[v.]"} $3 == 30 {print $0}')
if [[ ${#should_use_access} -gt 0 ]] && [ -e ${main_cfg_dir}/uplink/access.txt ]
then
    # the access provided by storj-sim uses an empty encryption key; we cannot do uplink setup above with an empty encryption key
    # therefore, we use a hack -> get an access key from the existing uplink config, then import that same access key

    # super hack:
    access=$(head -n 1 ${main_cfg_dir}/uplink/access.txt)
    new_access=$(go run $update_access_script_path $(storj-sim --config-dir=$main_cfg_dir network env SATELLITE_0_DIR) $access)
    uplink import --config-dir="${main_cfg_dir}/uplink" "${new_access}"
    rm -rf ${main_cfg_dir}/uplink/access.txt
fi

echo -e "\nConfig directory for satellite:"
echo "${main_cfg_dir}/satellite/0"
echo "Shasum for satellite:"
shasum ${main_cfg_dir}/satellite/0/satellite

echo -e "\nStoragenode config directories:"
for i in {0..9}
do
    echo -e "\nConfig directory for sn ${i}:"
    echo "${main_cfg_dir}/storagenode/${i}"
    echo "Shasum for sn ${i} binary:"
    shasum ${main_cfg_dir}/storagenode/${i}/storagenode
done

if [[ "$command" == "upload" ]]; then
    setup
    bucket_name=${bucket}-${uplink_version}
    download_dst_dir=${stage1_dst_dir}/${uplink_version}
    mkdir -p "$download_dst_dir"

    uplink mb "sj://$bucket_name/" --config-dir="${main_cfg_dir}/uplink"

    # run each upload in parallel
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/small-upload-testfile" "sj://$bucket_name/" &
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/big-upload-testfile" "sj://$bucket_name/" &
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/multisegment-upload-testfile" "sj://$bucket_name/" &
    wait_for_all_background_jobs_to_finish

    # run each download in parallel
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}" &
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}" &
    uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}" &
    wait_for_all_background_jobs_to_finish

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

    rm -rf ${test_files_dir}
fi

if [[ "$command" == "download" ]]; then
    existing_bucket_name_suffixes=$5

    # download all uploaded files from stage 1 with currently selected uplink
    for suffix in ${existing_bucket_name_suffixes}; do
        bucket_name=${bucket}-${suffix}
        original_dst_dir=${stage1_dst_dir}/${suffix}
        download_dst_dir=${stage2_dst_dir}/${suffix}
        mkdir -p "$download_dst_dir"

        echo "bucket name: ${bucket_name}"
        echo "download folder name: ${download_dst_dir}"
        # run each download in parallel
        uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}" &
        uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}" &
        uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}" &
        wait_for_all_background_jobs_to_finish

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

        rm -rf ${download_dst_dir}
    done
fi

if [[ "$command" == "cleanup" ]]; then
    uplink_versions=$3
    for ul_version in ${uplink_versions}; do
        bucket_name=${bucket}-${ul_version}
        uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/small-upload-testfile"
        uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/big-upload-testfile"
        uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/multisegment-upload-testfile"
        uplink rb --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name"
    done
fi

echo "Done with test-versions.sh"
