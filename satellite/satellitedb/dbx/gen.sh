#!/bin/sh
set -ueo pipefail

FILES=()
for f in *.dbx
do
  if [ "$f" == "satellitedb.dbx" ] ; then
    continue;
  fi
  args+=(-i "$f")
done
FILES="${args[@]}"

dbx schema -d pgx -d pgxcockroach $FILES satellitedb.dbx .
dbx golang -d pgx -d pgxcockroach -p dbx -t templates $FILES satellitedb.dbx .
( printf '%s\n' '//lint:file-ignore U1000,ST1012 generated file'; cat satellitedb.dbx.go ) > satellitedb.dbx.go.tmp && mv satellitedb.dbx.go.tmp satellitedb.dbx.go
gofmt -r "*sql.Tx -> tagsql.Tx" -w satellitedb.dbx.go
gofmt -r "*sql.Rows -> tagsql.Rows" -w satellitedb.dbx.go
perl -0777 -pi \
  -e 's,\t_ "github.com/jackc/pgx/v4/stdlib"\n\),\t_ "github.com/jackc/pgx/v4/stdlib"\n\n\t"storj.io/private/tagsql"\n\),' \
  satellitedb.dbx.go
perl -0777 -pi \
  -e 's/type DB struct \{\n\t\*sql\.DB/type DB struct \{\n\ttagsql.DB/' \
  satellitedb.dbx.go
perl -0777 -pi \
  -e 's/\tdb = &DB\{\n\t\tDB: sql_db,/\tdb = &DB\{\n\t\tDB: tagsql.Wrap\(sql_db\),/' \
  satellitedb.dbx.go
