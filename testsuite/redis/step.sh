#!/usr/bin/env bash
set -Eeo pipefail
set +x

# constants
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
source $SCRIPTDIR/../../scripts/utils.sh


readonly SCRIPTDIR
BUCKET="bucket-123"
readonly BUCKET
UPLINK_DEBUG_ADDR=""
readonly UPLINK_DEBUG_ADDR

export STORJ_ACCESS="${GATEWAY_0_ACCESS}"
export UPLINK_ACCESS="${STORJ_ACCESS}"
export STORJ_DEBUG_ADDR="${UPLINK_DEBUG_ADDR}"

# Vars
temp_dirs=() # used to track all the created temporary directories

cleanup() {
	trap - EXIT

	rm -rf "${temp_dirs[@]}"
	echo "cleaned up test successfully"
}
trap cleanup EXIT
trap 'failure ${LINENO} "$BASH_COMMAND"' ERR

uplink_test() {
	local temp_dir
	temp_dir=$(mktemp -d -t tmp.XXXXXXXXXX)
	temp_dirs+=("${temp_dir}")

	local src_dir="${temp_dir}/source"
	local dst_dir="${temp_dir}/dst"
	mkdir -p "${src_dir}" "${dst_dir}"

	local UPLINK_CONFIG_DIR="${temp_dir}/uplink"
	export UPLINK_CONFIG_DIR
	mkdir -p "$UPLINK_CONFIG_DIR"
	touch "$UPLINK_CONFIG_DIR/config.ini"

	random_bytes_file "2KiB" "${src_dir}/small-upload-testfile" # create 2KiB file of random bytes (inline)
	random_bytes_file "5MiB" "${src_dir}/big-upload-testfile"   # create 5MiB file of random bytes (remote)
	# this is special case where we need to test at least one remote segment and inline segment of exact size 0
	random_bytes_file "12MiB" "${src_dir}/multisegment-upload-testfile" # create 12MiB file of random bytes (1 remote segments + inline)
	random_bytes_file "13MiB" "${src_dir}/diff-size-segments"           # create 13MiB file of random bytes (2 remote segments)

	random_bytes_file "100KiB" "${src_dir}/put-file" # create 100KiB file of random bytes (remote)

	uplink mb "sj://$BUCKET/"
	uplink cp "${src_dir}/small-upload-testfile" "sj://$BUCKET/" --progress=false
	uplink cp "${src_dir}/big-upload-testfile" "sj://$BUCKET/" --progress=false
	uplink cp "${src_dir}/multisegment-upload-testfile" "sj://$BUCKET/" --progress=false
	uplink cp "${src_dir}/diff-size-segments" "sj://$BUCKET/" --progress=false

	uplink <"${src_dir}/put-file" cp - "sj://$BUCKET/put-file"

	uplink access import -f named-access "${STORJ_ACCESS}"

	local files
	files=$(uplink ls "sj://${BUCKET}" --access named-access | grep -v '^KIND' | tee "${temp_dir}/list" | wc -l)
	local expected_files="5"
	if [ "${files}" == "${expected_files}" ]; then
		echo "listing returns ${files} files"
	else
		echo "listing returns ${files} files but want ${expected_files}"
		exit 1
	fi

	local size_check
	size_check=$(awk <"${temp_dir}/list" '{if($3 == "0") print "invalid size";}')
	if [ "${size_check}" != "" ]; then
		echo "listing returns invalid size for one of the objects:"
		cat "${temp_dir}/list"
		exit 1
	fi

	uplink ls "sj://$BUCKET/non-existing-prefix"

	uplink cp "sj://$BUCKET/small-upload-testfile" "${dst_dir}" --progress=false
	uplink cp "sj://$BUCKET/big-upload-testfile" "${dst_dir}" --progress=false
	uplink cp "sj://$BUCKET/multisegment-upload-testfile" "${dst_dir}" --progress=false
	uplink cp "sj://$BUCKET/diff-size-segments" "${dst_dir}" --progress=false
	uplink cp "sj://$BUCKET/put-file" "${dst_dir}" --progress=false
	uplink cp "sj://$BUCKET/put-file" - >>"${dst_dir}/put-file-from-cat"

	uplink rm "sj://$BUCKET/small-upload-testfile"
	uplink rm "sj://$BUCKET/big-upload-testfile"
	uplink rm "sj://$BUCKET/multisegment-upload-testfile"
	uplink rm "sj://$BUCKET/diff-size-segments"
	uplink rm "sj://$BUCKET/put-file"

	uplink ls "sj://$BUCKET"

	uplink rb "sj://$BUCKET"

	compare_files "${src_dir}/small-upload-testfile" "${dst_dir}/small-upload-testfile"
	compare_files "${src_dir}/big-upload-testfile" "${dst_dir}/big-upload-testfile"
	compare_files "${src_dir}/multisegment-upload-testfile" "${dst_dir}/multisegment-upload-testfile"
	compare_files "${src_dir}/diff-size-segments" "${dst_dir}/diff-size-segments"
	compare_files "${src_dir}/put-file" "${dst_dir}/put-file"
	compare_files "${src_dir}/put-file" "${dst_dir}/put-file-from-cat"

	# test deleting non empty bucket with --force flag
	uplink mb "sj://$BUCKET/"

	for i in $(seq -w 1 16); do
		uplink cp "${src_dir}/small-upload-testfile" "sj://$BUCKET/small-file-$i" --progress=false
	done

	uplink rb "sj://$BUCKET" --force

	if [ "$(uplink ls | grep -c '^CREATED')" != "0" ]; then
		echo "uplink didn't remove the entire bucket with the 'force' flag"
		exit 1
	fi
}

# Run the test with Redis container running
uplink_test

# Run the test with Redis container not running
"${SCRIPTDIR}/redis-server.sh" stop
uplink_test
