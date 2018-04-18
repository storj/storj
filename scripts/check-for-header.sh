#!/bin/bash

FILES=$(find $PWD -type f \( -iname '*.go' ! -iname "*.pb.go" \) )
for i in $FILES
do
  if ! grep -q 'Copyright' "$i"
  then
    echo " missing copyrioght header for $i"
  fi
done
