#!/bin/bash

CHANGES=$(grep -r --include="*.dbx.go" regexp.MustCompile .)

if [ -z "$CHANGES" ]
then
    echo "dbx version ok"
else
    echo "please use latest dbx tool to generate code"
fi