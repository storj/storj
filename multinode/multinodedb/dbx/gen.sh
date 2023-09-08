#!/bin/sh
set -e pipefail

dbx schema -d pgx -d sqlite3 multinodedb.dbx .
dbx golang -d pgx -d sqlite3 --rx=false -p dbx -t templates multinodedb.dbx .

( printf '%s\n' '//lint:file-ignore U1000,ST1012,SA1019 generated file'; cat multinodedb.dbx.go ) > multinodedb.dbx.go.tmp && mv multinodedb.dbx.go.tmp multinodedb.dbx.go
gofmt -r "*sql.Tx -> tagsql.Tx" -w multinodedb.dbx.go
gofmt -r "*sql.Rows -> tagsql.Rows" -w multinodedb.dbx.go
perl -0777 -pi \
  -e 's,\t"fmt"\n,\t"fmt"\n\t"math/rand"\n,' \
  multinodedb.dbx.go
perl -0777 -pi \
  -e 's,\t"math/rand"\n\),\n\t"storj.io/private/tagsql"\n\),' \
  multinodedb.dbx.go
perl -0777 -pi \
  -e 's/type DB struct \{\n\t\*sql\.DB/type DB struct \{\n\ttagsql.DB/' \
  multinodedb.dbx.go
perl -0777 -pi \
  -e 's/\tdb = &DB\{\n\t\tDB: sql_db,/\tdb = &DB\{\n\t\tDB: tagsql.Wrap\(sql_db\),/' \
  multinodedb.dbx.go
