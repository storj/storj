#!/bin/bash

FILES=$(find $PWD -type f \( -iname '*.go' ! -iname "*.pb.go" \) )
for i in $FILES
do
  if ! grep -q 'Copyright' "$i"
  then
    echo " missing copyright header for $i"
  fi
done
