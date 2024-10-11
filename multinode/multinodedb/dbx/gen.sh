#!/bin/sh
set -e pipefail

dbx schema -d pgx -d sqlite3 multinodedb.dbx .
dbx golang -d pgx -d sqlite3 -p dbx -t templates multinodedb.dbx .

( printf '%s\n' '//lint:file-ignore U1000 generated file'; cat multinodedb.dbx.go ) > multinodedb.dbx.go.tmp && mv multinodedb.dbx.go.tmp multinodedb.dbx.go
gofmt -r "*sql.Tx -> tagsql.Tx" -w multinodedb.dbx.go
gofmt -r "*sql.Rows -> tagsql.Rows" -w multinodedb.dbx.go
perl -0777 -pi -e "s,\"crypto/rand\",\"crypto/rand\"\n\t\"storj\.io\/storj\/shared\/tagsql\"," multinodedb.dbx.go
perl -0777 -pi -e 's/\*sql\.DB/tagsql.DB/' multinodedb.dbx.go
perl -0777 -pi -e 's/\tdb = &DB\{\n\t\tDB: sql_db,/\tdb = &DB\{\n\t\tDB: tagsql.Wrap\(sql_db\),/' multinodedb.dbx.go

goimports -w -local storj.io multinodedb.dbx.go