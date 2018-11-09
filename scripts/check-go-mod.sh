#/bin/bash

cat go.mod
cp go.mod go.backup.mod

echo "-----------"

go mod tidy -v

echo "-----------"

cp go.backup.mod go.mod 
go mod tidy -v

echo "-----------"

cp go.backup.mod go.mod 
go mod tidy -v 2>&1 | grep "^unused"

echo "-----------"

cp go.backup.mod go.mod 
if go mod tidy -v 2>&1 | grep "^unused" ; then
    exit 1
fi