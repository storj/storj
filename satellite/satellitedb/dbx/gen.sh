#!/bin/bash

dbx schema -d postgres -d cockroach satellitedb.dbx .
dbx golang -d postgres -d cockroach -t templates satellitedb.dbx .
( echo '//lint:file-ignore * generated file'; cat satellitedb.dbx.go ) > satellitedb.dbx.go.tmp && mv satellitedb.dbx.go{.tmp,}
gofmt -r "*sql.Tx -> dbwrap.Tx" -w satellitedb.dbx.go
perl -0777 -pi \
  -e 's,\t"github.com/lib/pq"\n\),\t"github.com/lib/pq"\n\n\t"storj.io/storj/private/dbutil/dbwrap"\n\),' \
  satellitedb.dbx.go
perl -0777 -pi \
  -e 's/type DB struct \{\n\t\*sql\.DB/type DB struct \{\n\tdbwrap.DB/' \
  satellitedb.dbx.go
perl -0777 -pi \
  -e 's/\tdb = &DB\{\n\t\tDB: sql_db,/\tdb = &DB\{\n\t\tDB: dbwrap.SQLDB\(sql_db\),/' \
  satellitedb.dbx.go
