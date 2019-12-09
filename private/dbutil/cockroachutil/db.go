// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"database/sql"
	"net/url"
	"strings"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/private/dbutil"
)

var mon = monkit.Package()

// OpenUnique opens a temporary unique CockroachDB database that will be cleaned up when closed.
// It is expected that this should normally be used by way of
// "storj.io/storj/private/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(connStr string, schemaName string) (db *dbutil.TempDatabase, err error) {
	if !strings.HasPrefix(connStr, "cockroach://") {
		return nil, errs.New("expected a cockroachDB URI, but got %q", connStr)
	}
	connStr = "postgres://" + connStr[12:]
	masterDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, masterDB.Close())
	}()
	err = masterDB.Ping()
	if err != nil {
		return nil, errs.New("Could not open masterDB at conn %q: %v", connStr, err)
	}

	_, err = masterDB.Exec("CREATE DATABASE " + pq.QuoteIdentifier(schemaName))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cleanup := func(cleanupDB *sql.DB) error {
		_, err := cleanupDB.Exec("DROP DATABASE " + pq.QuoteIdentifier(schemaName))
		return errs.Wrap(err)
	}

	modifiedConnStr, err := changeDBTargetInConnStr(connStr, schemaName)
	if err != nil {
		return nil, errs.Combine(err, cleanup(masterDB))
	}

	sqlDB, err := sql.Open("postgres", modifiedConnStr)
	if err != nil {
		return nil, errs.Combine(errs.Wrap(err), cleanup(masterDB))
	}

	dbutil.Configure(sqlDB, mon)
	return &dbutil.TempDatabase{
		DB:             sqlDB,
		ConnStr:        modifiedConnStr,
		Schema:         schemaName,
		Driver:         "postgres",
		Implementation: dbutil.Cockroach,
		Cleanup:        cleanup,
	}, nil
}

func changeDBTargetInConnStr(connStr string, newDBName string) (string, error) {
	connURL, err := url.Parse(connStr)
	if err != nil {
		return "", errs.Wrap(err)
	}
	connURL.Path = newDBName
	return connURL.String(), nil
}
