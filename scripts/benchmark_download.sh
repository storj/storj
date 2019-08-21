#!/bin/bash

usage="Usage: $(basename "$0") <file_to_upload> <s3_bucket> <storj_bucket> <operations>"

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

if [ "$4" == "" ]; then
    echo "$usage" >&2
    exit 1
fi

# SETTINGS
UPLOAD_FILE=$1
DOWNLOAD_DIR="/tmp/"
S3_BUCKET=$2
STORJ_BUCKET=$3
OPERATIONS=$4
FILENAME="$(basename -- $UPLOAD_FILE)"
DOWNLOAD_FILE="$DOWNLOAD_DIR$FILENAME"
S3_LOG_FILE="s3_download.log"
S3_RESULTS_FILE="s3_download_results.log"
STORJ_LOG_FILE="storj_download.log"
STORJ_RESULTS_FILE="storj_download_results.log"
S3_DOWNLOAD_FAILURES=0
S3_CHECKSUM_FAILURES=0
STORJ_DOWNLOAD_FAILURES=0
STORJ_CHECKSUM_FAILURES=0

# Upload benchmark files.
echo "Uploading file to S3..."
( aws s3 cp $UPLOAD_FILE s3://$S3_BUCKET )
echo "Uploading file to Storj..."
( uplink --log.level error --log.output /tmp/storj.log cp $UPLOAD_FILE sj://$STORJ_BUCKET/$FILENAME )

# S3 download benchmark.
echo "Benchmarking S3 Download..."
rm -rf "$S3_LOG_FILE"
rm -rf "$S3_RESULTS_FILE"
for (( i=1; i<=$OPERATIONS; i++ ))
do
    rm -rf $DOWNLOAD_FILE
    response="$(/usr/bin/time -p aws s3 cp s3://$S3_BUCKET/$FILENAME $DOWNLOAD_FILE 2>&1)"
    if [ $? == 0 ]; then
        response_time=`echo "$response" | awk '/real/{print $2}'`
        cmp --silent $UPLOAD_FILE $DOWNLOAD_FILE || echo "files are different"
        if [ $? == 0 ]; then
            echo $response_time >> "$S3_LOG_FILE"
        else
            echo "S3: failed checksum"
            let "STORJ_CHECKSUM_FAILURES++"
        fi        
    else
        echo "S3: failed to download file"
        let "STORJ_DOWNLOAD_FAILURES++"
    fi
done

latency_50="$(cat $S3_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.50-0.5)]}')"
latency_75="$(cat $S3_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.75-0.5)]}')"
latency_90="$(cat $S3_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.90-0.5)]}')"
latency_95="$(cat $S3_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.95-0.5)]}')"
latency_99="$(cat $S3_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.99-0.5)]}')"

cat >$S3_RESULTS_FILE <<EOL
50%: ${latency_50}s
75%: ${latency_75}s
90%: ${latency_90}s
95%: ${latency_95}s
99%: ${latency_99}s
EOL
echo "Download failures: $S3_DOWNLOAD_FAILURES" >> $S3_RESULTS_FILE
echo "Checksum failures: $S3_CHECKSUM_FAILURES" >> $S3_RESULTS_FILE


# Storj download benchmark.
echo "Benchmarking Storj Download..."

rm -rf "$STORJ_LOG_FILE"
rm -rf "$STORJ_RESULTS_FILE"
for (( i=1; i<=$OPERATIONS; i++ ))
do
    rm -rf $DOWNLOAD_FILE
    response="$(/usr/bin/time -p uplink --log.level error --log.output /tmp/storj.log cp sj://$STORJ_BUCKET/$FILENAME $DOWNLOAD_FILE 2>&1)"
    if [ $? == 0 ]; then
        response_time=`echo "$response" | awk '/real/{print $2}'`
        cmp --silent $UPLOAD_FILE $DOWNLOAD_FILE || echo "files are different"
        if [ $? == 0 ]; then
            echo $response_time >> "$STORJ_LOG_FILE"
        else
            echo "STORJ: failed checksum"
            let "STORJ_CHECKSUM_FAILURES++"
        fi        
    else
        echo "STORJ: failed to download file"
        let "STORJ_DOWNLOAD_FAILURES++"
    fi
done

latency_50="$(cat $STORJ_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.50-0.5)]}')"
latency_75="$(cat $STORJ_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.75-0.5)]}')"
latency_90="$(cat $STORJ_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.90-0.5)]}')"
latency_95="$(cat $STORJ_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.95-0.5)]}')"
latency_99="$(cat $STORJ_LOG_FILE| sort -n | awk 'BEGIN{i=0} {s[i]=$1; i++;} END{print s[int(NR*0.99-0.5)]}')"

cat >$STORJ_RESULTS_FILE <<EOL
50%: ${latency_50}s
75%: ${latency_75}s
90%: ${latency_90}s
95%: ${latency_95}s
99%: ${latency_99}s
EOL
echo "Download failures: $STORJ_DOWNLOAD_FAILURES" >> $STORJ_RESULTS_FILE
echo "Checksum failures: $STORJ_CHECKSUM_FAILURES" >> $STORJ_RESULTS_FILE

rm -rf $DOWNLOAD_FILE