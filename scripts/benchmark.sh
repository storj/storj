#!/bin/bash

# Copyright (C) 2019 Storj Labs, Inc.
# See LICENSE for copying information.

usage="Usage: $(basename "$0") <service> <command> <file_name> <bucket> <operations>"

if [ "$1" == "" ]; then
    echo "$usage" >&2
    exit 1
fi
if [ "$2" == "" ]; then
    echo "$usage" >&2
    exit 1
fi
if [ "$3" == "" ]; then
    echo "$usage" >&2
    exit 1
fi

# SETTINGS
SERVICE=$1
COMMAND=$2
FILE=$3
BUCKET=$4
OPERATIONS=$5
FILENAME="$(basename -- $FILE)"
DOWNLOAD_DIR="/tmp/"
DOWNLOAD_FILE="$DOWNLOAD_DIR$FILENAME"
EXEC_COMMAND=""
UPLOAD_COMMAND=""
DOWNLOAD_COMMAND=""
LOG_FILE=$SERVICE"_"$COMMAND".log"
RESULTS_FILE=$SERVICE"_"$COMMAND"_results.log"
FAILURES=0

if [ "$SERVICE" == "aws" ]; then
    UPLOAD_COMMAND="exec_aws_upload"
    DOWNLOAD_COMMAND="exec_aws_download"
elif [ "$SERVICE" == "storj" ]; then
    UPLOAD_COMMAND="exec_storj_upload"
    DOWNLOAD_COMMAND="exec_storj_download"
fi

if [ "$COMMAND" == "upload" ]; then
    EXEC_COMMAND=$UPLOAD_COMMAND
elif [ "$COMMAND" == "download" ]; then
    EXEC_COMMAND=$DOWNLOAD_COMMAND
fi

function exec_aws_upload() {
    /usr/bin/time aws s3 cp $FILE s3://$BUCKET 2>&1 | tail -n 3
}
function exec_aws_download() {
    /usr/bin/time -p aws s3 cp s3://$BUCKET/$FILENAME $DOWNLOAD_FILE 2>&1 | tail -n 3
}

function exec_storj_create_bucket() {
    /usr/bin/time -p uplink --log.level error --log.output /tmp/storj.log mb sj://$BUCKET 2>&1
    return
}

function exec_storj_upload() {    
    /usr/bin/time -p uplink --log.level error --log.output /tmp/storj.log cp $FILE sj://$BUCKET/$FILENAME 2>&1 | tail -n 3
}
function exec_storj_download() {
    /usr/bin/time -p uplink --log.level error --log.output /tmp/storj.log cp sj://$BUCKET/$FILENAME $DOWNLOAD_FILE 2>&1 | tail -n 3
}


echo "========================================"
echo "Environment"
echo "========================================"
echo "UPLOAD CMD:" $UPLOAD_COMMAND
echo "DOWNLOAD CMD:" $DOWNLOAD_COMMAND
echo "EXEC CMD:" $EXEC_COMMAND
echo "BUCKET:" $BUCKET
echo "FILE:" $FILE
echo "OPERATIONS:" $OPERATIONS
echo "LOG FILE:" $LOG_FILE
echo "RESULTS FILE:" $RESULTS_FILE
echo ""
echo "========================================"
echo "Benchmark"
echo "========================================"

if [ "$COMMAND" == "download" ]; then
    echo "Uploading file for download benchmark..."
    exec_storj_create_bucket
    $UPLOAD_COMMAND  
fi

# Benchmark.
echo "Benchmarking $SERVICE $COMMAND..."
rm -rf "$LOG_FILE"
rm -rf "$RESULTS_FILE"
for (( i=1; i<=$OPERATIONS; i++ ))
do
    response="$(${EXEC_COMMAND})"
    if [ $? == 0 ]; then
        response_time=`echo "$response" | awk '/real/{print $2}'`        
        echo $response_time >> "$LOG_FILE"
    else
        echo "Failed to "$COMMAND" file"
        echo $response
        let "FAILURES++"
    fi
done

latency_50="$(cat $LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.50-0.5)]}')"
latency_75="$(cat $LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.75-0.5)]}')"
latency_90="$(cat $LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.90-0.5)]}')"
latency_95="$(cat $LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.95-0.5)]}')"
latency_99="$(cat $LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.99-0.5)]}')"

cat >$RESULTS_FILE <<EOL
50%: ${latency_50}s
75%: ${latency_75}s
90%: ${latency_90}s
95%: ${latency_95}s
99%: ${latency_99}s
EOL
echo "Failures: $FAILURES" >> $RESULTS_FILE

cat $RESULTS_FILE
