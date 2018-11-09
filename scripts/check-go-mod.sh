#/bin/bash

cat go.mod

if go mod tidy -v 2>&1 | grep "^unused" ; then
    exit 1
fi