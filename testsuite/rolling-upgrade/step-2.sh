#!/usr/bin/env bash

# This file contains the second part of Stage 2 for the rolling upgrade test.
# Description of file functionality:
#  * Upload an inline, remote, and multisegment file to the network using the main uplink and the new satellite api.
#  * Upload an inline, remote, and multisegment file to the network using the main uplink and the old satellite api.
#  * Download the six inline, remote, and multisegment files from the previous two steps using the main uplink and new satellite api.
#  * Download the six inline, remote, and multisegment files from the previous two steps using the main uplink and old satellite api.

set -ueo pipefail

# This script assumes that storj-sim and uplink has already been set up
main_cfg_dir=$1

bucket="bucket-123"
test_files_dir="${main_cfg_dir}/testfiles"
stage2_dst_dir="${main_cfg_dir}/stage2"

create_test_files(){
    mkdir -p "$test_files_dir"
    random_bytes_file () {
        size=$1
        output=$2
	    head -c $size </dev/urandom > $output
    }
    random_bytes_file "2048"   "$test_files_dir/small-upload-testfile"          # create 2kb file of random bytes (inline)
    random_bytes_file "5120" "$test_files_dir/big-upload-testfile"              # create 5kb file of random bytes (remote)
    random_bytes_file "131072" "$test_files_dir/multisegment-upload-testfile"   # create 128kb file of random bytes (remote)

    echo "created test files"
}

create_test_files

# Test that new files can be uploaded and downloaded successfully with both apis
bucket_name=${bucket}-final-upload
old_api_bucket_name=${bucket}-final-upload-old
# download directory for new-api-uploaded + new-api-downloaded files
download_dst_dir=${stage2_dst_dir}/final-upload
# download directory for old-api-uploaded + new-api-downloaded files
download_dst_dir2=${stage2_dst_dir}/final-upload2
# download directory for new-api-uploaded + old-api-downloaded files
old_api_download_dst_dir=${download_dst_dir}/old-api
# download directory for old-api-uploaded + old-api-downloaded files
old_api_download_dst_dir2=${download_dst_dir2}/old-api
mkdir -p "$download_dst_dir"
mkdir -p "$download_dst_dir2"
mkdir -p "$old_api_download_dst_dir"
mkdir -p "$old_api_download_dst_dir2"

uplink mb "sj://$bucket_name/" --config-dir="${main_cfg_dir}/uplink"
uplink mb "sj://$old_api_bucket_name/" --config-dir="${main_cfg_dir}/uplink-old-api"

# new api uploads
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/small-upload-testfile" "sj://$bucket_name/"
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/big-upload-testfile" "sj://$bucket_name/"
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "${test_files_dir}/multisegment-upload-testfile" "sj://$bucket_name/"

# TODO we should be able to uncomment those cases when we will have at least one point release of multipart satellite after merging to main
# old api uploads
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "${test_files_dir}/small-upload-testfile" "sj://$old_api_bucket_name/"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "${test_files_dir}/big-upload-testfile" "sj://$old_api_bucket_name/"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "${test_files_dir}/multisegment-upload-testfile" "sj://$old_api_bucket_name/"

# new api downloads of new api uploaded files
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/small-upload-testfile" "${download_dst_dir}"
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/big-upload-testfile" "${download_dst_dir}"
uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${download_dst_dir}"

# TODO we should be able to uncomment those cases when we will have at least one point release of multipart satellite after merging to main
# new api downloads of old api uploaded files
# uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$old_api_bucket_name/small-upload-testfile" "${download_dst_dir2}"
# uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$old_api_bucket_name/big-upload-testfile" "${download_dst_dir2}"
# uplink cp --config-dir="${main_cfg_dir}/uplink" --progress=false "sj://$old_api_bucket_name/multisegment-upload-testfile" "${download_dst_dir2}"

echo "checking files uploaded with new api and downloaded with new api"
if cmp "${test_files_dir}/small-upload-testfile" "${download_dst_dir}/small-upload-testfile"
then
    echo "download test on current branch: small upload testfile matches uploaded file"
else
    echo "download test on current branch: small upload testfile does not match uploaded file"
    exit 1
fi
if cmp "${test_files_dir}/big-upload-testfile" "${download_dst_dir}/big-upload-testfile"
then
    echo "download test on current branch: big upload testfile matches uploaded file"
else
    echo "download test on current branch: big upload testfile does not match uploaded file"
    exit 1
fi
if cmp "${test_files_dir}/multisegment-upload-testfile" "${download_dst_dir}/multisegment-upload-testfile"
then
    echo "download test on current branch: multisegment upload testfile matches uploaded file"
else
    echo "download test on current branch: multisegment upload testfile does not match uploaded file"
    exit 1
fi

# TODO we should be able to uncomment those cases when we will have at least one point release of multipart satellite after merging to main
# echo "checking files uploaded with old api and downloaded with new api"
# if cmp "${test_files_dir}/small-upload-testfile" "${download_dst_dir2}/small-upload-testfile"
# then
#     echo "download test on current branch: small upload testfile matches uploaded file"
# else
#     echo "download test on current branch: small upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/big-upload-testfile" "${download_dst_dir2}/big-upload-testfile"
# then
#     echo "download test on current branch: big upload testfile matches uploaded file"
# else
#     echo "download test on current branch: big upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/multisegment-upload-testfile" "${download_dst_dir2}/multisegment-upload-testfile"
# then
#     echo "download test on current branch: multisegment upload testfile matches uploaded file"
# else
#     echo "download test on current branch: multisegment upload testfile does not match uploaded file"
#     exit 1
# fi

# TODO we should be able to uncomment those cases when we will have at least one point release of multipart satellite after merging to main
# old api downloads of new api uploaded files
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$bucket_name/small-upload-testfile" "${old_api_download_dst_dir}"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$bucket_name/big-upload-testfile" "${old_api_download_dst_dir}"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$bucket_name/multisegment-upload-testfile" "${old_api_download_dst_dir}"
# old api downloads of old api uploaded files
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$old_api_bucket_name/small-upload-testfile" "${old_api_download_dst_dir2}"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$old_api_bucket_name/big-upload-testfile" "${old_api_download_dst_dir2}"
# uplink cp --config-dir="${main_cfg_dir}/uplink-old-api" --progress=false "sj://$old_api_bucket_name/multisegment-upload-testfile" "${old_api_download_dst_dir2}"

# echo "checking files uploaded with new api and downloaded with old api"
# if cmp "${test_files_dir}/small-upload-testfile" "${old_api_download_dst_dir}/small-upload-testfile"
# then
#     echo "download test on current branch: small upload testfile matches uploaded file"
# else
#     echo "download test on current branch: small upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/big-upload-testfile" "${old_api_download_dst_dir}/big-upload-testfile"
# then
#     echo "download test on current branch: big upload testfile matches uploaded file"
# else
#     echo "download test on current branch: big upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/multisegment-upload-testfile" "${old_api_download_dst_dir}/multisegment-upload-testfile"
# then
#     echo "download test on current branch: multisegment upload testfile matches uploaded file"
# else
#     echo "download test on current branch: multisegment upload testfile does not match uploaded file"
#     exit 1
# fi

# echo "checking files uploaded with old api and downloaded with old api"
# if cmp "${test_files_dir}/small-upload-testfile" "${old_api_download_dst_dir2}/small-upload-testfile"
# then
#     echo "download test on current branch: small upload testfile matches uploaded file"
# else
#     echo "download test on current branch: small upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/big-upload-testfile" "${old_api_download_dst_dir2}/big-upload-testfile"
# then
#     echo "download test on current branch: big upload testfile matches uploaded file"
# else
#     echo "download test on current branch: big upload testfile does not match uploaded file"
#     exit 1
# fi
# if cmp "${test_files_dir}/multisegment-upload-testfile" "${old_api_download_dst_dir2}/multisegment-upload-testfile"
# then
#     echo "download test on current branch: multisegment upload testfile matches uploaded file"
# else
#     echo "download test on current branch: multisegment upload testfile does not match uploaded file"
#     exit 1
# fi

rm -rf ${download_dst_dir}
rm -rf ${download_dst_dir2}

uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/small-upload-testfile"
uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/big-upload-testfile"
uplink rm --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name/multisegment-upload-testfile"
uplink rb --config-dir="${main_cfg_dir}/uplink" "sj://$bucket_name"

# TODO we should be able to uncomment those cases when we will have at least one point release of multipart satellite after merging to main
# uplink rm --config-dir="${main_cfg_dir}/uplink-old-api" "sj://$old_api_bucket_name/small-upload-testfile"
# uplink rm --config-dir="${main_cfg_dir}/uplink-old-api" "sj://$old_api_bucket_name/big-upload-testfile"
# uplink rm --config-dir="${main_cfg_dir}/uplink-old-api" "sj://$old_api_bucket_name/multisegment-upload-testfile"
# uplink rb --config-dir="${main_cfg_dir}/uplink-old-api" "sj://$old_api_bucket_name"
