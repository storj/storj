#!/bin/bash

FILES=$(find $PWD -type f \( -iname '*.go' ! -iname "*.pb.go" \) )
for i in $FILES
do
  if ! grep -q 'Copyright' <<< "$(head -n 2 "$i")" 
  then
    echo " missing copyright header for $i"
  fi
done
