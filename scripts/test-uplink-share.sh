#!/usr/bin/env bash

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    rm -rf "$TMPDIR"
    uplink rm "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile"
    uplink rm "sj://$BUCKET_WITH_ACCESS/another-testfile"
    uplink rm "sj://$BUCKET_WITHOUT_ACCESS/another-testfile"
    uplink rb "sj://$BUCKET_WITHOUT_ACCESS"
    uplink rb "sj://$BUCKET_WITH_ACCESS"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

BUCKET_WITHOUT_ACCESS=bucket1
BUCKET_WITH_ACCESS=bucket2

FOLDER_TO_SHARE_FILE=folder

SRC_DIR=$TMPDIR/source
DST_DIR=$TMPDIR/dst

mkdir -p "$SRC_DIR" "$DST_DIR"

random_bytes_file () {
    size=$1
    output=$2
    head -c $size </dev/urandom > $output
}

random_bytes_file "2048"       "$SRC_DIR/another-testfile"  # create 2kb file of random bytes (inline)
random_bytes_file "5120"       "$SRC_DIR/testfile"          # create 5kb file of random bytes (inline)
uplink mb "sj://$BUCKET_WITHOUT_ACCESS/"
uplink mb "sj://$BUCKET_WITH_ACCESS/"

uplink cp --progress=false "$SRC_DIR/testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/"
uplink cp --progress=false "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/"
uplink cp --progress=false "$SRC_DIR/another-testfile" "sj://$BUCKET_WITHOUT_ACCESS/"

# Make scope with readonly rights
scope=$(uplink share --allowed-path-prefix sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/ --readonly | grep scope | cut -d: -f2)

check_exit_code(){
	if [ $1 -eq 0 ]; then
    	echo "Result of copying does not match expectations. Test FAILED"
    	exit 1
	else
		echo "Copy file without permission: PASSED"    # Expect unsuccessful exit code
	fi
}

uplink cp --progress=false "$SRC_DIR/another-testfile" "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/" --scope $scope
retVal=$?
check_exit_code $retVal

uplink cp --progress=false "$SRC_DIR/testfile" "sj://$BUCKET_WITHOUT_ACCESS/" --scope $scope
retVal=$?
check_exit_code $retVal

uplink cp --progress=false "sj://$BUCKET_WITHOUT_ACCESS/another-testfile" "$SRC_DIR/" --scope $scope
retVal=$?
check_exit_code $retVal

number_of_buckets=$(uplink ls --scope $scope | wc -l)

# We share one bucket, so we expect to see only one bucket in the output of ls command
if [ $number_of_buckets -eq 1 ]; then
	echo "Number of shared buckets matches the expected result. PASSED"
else
	echo "List of buckets more than 1. FAILED"
	exit 1
fi

uplink cp --progress=false "sj://$BUCKET_WITH_ACCESS/$FOLDER_TO_SHARE_FILE/testfile" "$DST_DIR" --scope $scope

if cmp "$SRC_DIR/testfile" "$DST_DIR/testfile"; then
    echo "Testfile matches uploaded file: PASSED"
else
    echo "Download test: FAILED"
    exit 1
fi
