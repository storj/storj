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

# version_ge returns true if version $1 is greater than or equal to $2
version_ge(){
    [ "$( ( echo "$1"; echo "$2" ) | sort -V | head -n 1 )" = "$2" ]
}

replace_in_file(){
    local src="$1"
    local dest="$2"
    local path=$3
    case "$OSTYPE" in
    darwin*)
        sed -i '' "s|# ${src}|${dest}|" "${path}" ;;
    *)
        sed -i "s|# ${src}|${dest}|" "${path}" ;;
    esac
}

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

echo "Begin uplink-versions/steps.sh, storj-sim config directory:" ${main_cfg_dir}

echo "which storj-sim: $(which storj-sim)"
echo "Shasum for storj-sim:"
shasum $(which storj-sim)

echo -e "\nConfig directory for uplink:"
echo "${main_cfg_dir}/uplink"
echo "which uplink: $(which uplink)"
echo "Shasum for uplink:"
shasum $(which uplink)

export UPLINK_CONFIG_DIR="${main_cfg_dir}/uplink"
export UPLINK_LEGACY_CONFIG_DIR="${main_cfg_dir}/uplink"

if [ ! -d ${main_cfg_dir}/uplink ]; then
    mkdir -p ${main_cfg_dir}/uplink
    access=$(storj-sim --config-dir=$main_cfg_dir network env GATEWAY_0_ACCESS)
    new_access=$(go run $update_access_script_path $(storj-sim --config-dir=$main_cfg_dir network env SATELLITE_0_DIR) $access)
    echo "access: ${new_access}" > "${main_cfg_dir}/uplink/config.yaml"
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
        # skip downloads for uplink versions older than v1.27.6 against buckets
        # that are v1.48.0 or later because the newer uplinks always upload with
        # multipart uploads and older uplinks cannot download those.
        if [ "$uplink_version" != "main" ] && \
           ! version_ge "$uplink_version" "v1.27.6" && \
             version_ge "$suffix" "v1.48.0"; then
            echo "Skipping $uplink_version downloading $suffix"
            continue
        fi

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

echo "Done with uplink-versions/steps.sh"
