#!/bin/bash

clear
echo "==========================="
echo "Tool for quick storj test "
echo "==========================="
UPLOADFILE=""
DOWNLOADFILE=""

function installStorjRelease {
    shutdown
    cd $HOME
    echo "Current Working Directory -->" $PWD 
    if [ -d "storj" ]
    then
        echo "directory exists, deleting....."
        #rm -rf storj
    else
        echo "directory doesn't exists"
    fi

    read -p 'Enter the Release :' storjRelease
    echo "Downloading $storjRelease ...."
    #git clone https://github.com/storj/storj -b $storjRelease
    cd $HOME/storj
    echo "build & install storj ...."
    go install -v ./...
}

function runStorjSim {
    echo "Running storj-sim...."
    osascript -e 'tell app "Terminal" to do script "storj-sim network run"'
    sleep 2
}

function uplinkUpload() {
    local uplinkBucket
    local uplinkUploadFile
    echo "uplink uploading"
    echo "==========================="
    read -p 'Enter the bucket to upload to:' uplinkBucket
    read -p 'Enter the file path to upload:' uplinkUploadFile
    if [ -z "$uplinkUploadFile" ]; then
        echo "Empty file path entered ..."
    else
        if [ -f "$uplinkUploadFile" ]; then
            uplink cp $uplinkUploadFile sj://$uplinkBucket/
            UPLOADFILE=$uplinkUploadFile
        else
            echo "Invalid file path"
        fi
    fi
}

function uplinkDownload() {
    local uplinkBucket
    local uplinkDownloadFile
    local uplinkDownloadPath 
    echo "uplink downloading"
    echo "==========================="
    read -p 'Enter the bucket to download from:' uplinkBucket
    if [ -z "$uplinkBucket" ]; then
        echo "Empty bucket name entered ..."
    else
        read -p 'Enter the file to download:' uplinkDownloadFile 
        if [ -z "$uplinkDownloadFile" ]; then
            echo "Empty file name entered ..."
        else
            read -p 'Enter the path to save the downloaded file to:' uplinkDownloadPath
            if [ -z "$uplinkDownloadPath" ]; then
                echo "Empty file path entered ..."
            else
                if [ -f "$uplinkDownloadPath" ]; then
                    read -p 'File already exits. Overwrite? ... [Yy|Nn]:' overWrite
                    if [ "$overWrite" == "y" ] || [ "$overWrite" == "Y" ]; then
                        echo "overwriting ....."
                        uplink cp sj://$uplinkBucket/$uplinkDownloadFile $uplinkDownloadPath
                        DOWNLOADFILE=$uplinkDownloadPath
                    else
                        echo "Abort overwriting ....."
                    fi
                else
                    echo "Invalid file path"
                fi
            fi 
        fi
    fi
}

function compareFiles {
    echo "comparing files .... "
    if [ -z "$UPLOADFILE" ] || [ -z "$DOWNLOADFILE" ]; then
        echo
        echo "      file(s) are empty                           "
        echo
    else
        file1=$(shasum $UPLOADFILE | awk '{print $1}')
        file2=$(shasum $DOWNLOADFILE | awk '{print $1}')
        if [ "$file1" == "$file2" ]; then
            echo
            echo
            echo "**************************************************"
            echo "    Upload & Download files are *** SAME *** "
            echo "**************************************************"
            echo
            echo
        else
            echo
            echo
            echo "**************************************************"
            echo "  Upload & Download files are *** NOT SAME *** "
            echo "**************************************************"
            echo
            echo
        fi
    fi
}

function shutdown {
    storjSimPid=$(pgrep storj-sim)
    if [ -z "$storjSimPid" ]; then
        echo
    else
        echo killing storj-sim ... $storjSimPid
        kill -9 $storjSimPid
    fi
    # osascript -e 'tell application "Terminal" to quit'
}

all_done=0
while (( !all_done )); do
    options=("Download & build Storj Release" "Run Storj Sim" "Uplink Upload" "Uplink Download" "Compare Files" "Quit")

    PS3="Choose an option: "
    COLUMNS=12
    select opt in "${options[@]}"; do
    case $REPLY in
        1) installStorjRelease; break ;;
        2) runStorjSim; break ;;
        3) uplinkUpload; break ;;
        4) uplinkDownload; break ;;
        5) compareFiles; break ;;
        6) all_done=1; break ;;
        *) echo "Invalid option" ;;
    esac
    done
done

echo "Exiting ...." 
shutdown
sleep 2