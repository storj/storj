#!/bin/bash

# osx: $ ./print-capt-ids.sh ~/Library/Application\ Support/Storj/Capt/
basepath=$1
i=0
while [ $i -le 99 ]
do
  identity ca id --ca.cert-path "${basepath}/f${i}/ca.cert"
  ((i++))
done

