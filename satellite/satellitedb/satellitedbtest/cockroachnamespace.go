// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"fmt"
	"regexp"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// NewCockroach creates a new satellite.DB that is used for testing. We create a new database with a
// unique name so that there aren't conflicts when we run tests (since we may run the tests in parallel).
// Postgres supports schemas for namespacing, but cockroachdb doesn't, so instead we use a different database for each test.
func NewCockroach(log *zap.Logger, namespacedTestDB string) (satellite.DB, error) {
	if err := CockroachDefined(); err != nil {
		return nil, err
	}

	driver, source, err := dbutil.SplitConnstr(*pgtest.CrdbConnStr)
	if err != nil {
		return nil, err
	}
	db, err := dbx.Open(driver, source)
	if err != nil {
		return nil, err
	}

	r := regexp.MustCompile(`\W`)
	// this regex removes any non-alphanumeric character from the string
	namespacedTestDB = r.ReplaceAllString(namespacedTestDB, "")
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s;", pq.QuoteIdentifier(namespacedTestDB)))
	if err != nil {
		return nil, err
	}

	// this regex matches substrings like this "/dbName?"
	r = regexp.MustCompile("[/][a-zA-Z0-9]+[?]")
	if !r.MatchString(source) {
		return nil, errs.New("expecting db url format to contain a substring like '/dbName?', but got %s", source)
	}
	testConnURL := r.ReplaceAllString(source, "/"+namespacedTestDB+"?")
	testDB, err := satellitedb.New(log, testConnURL)
	if err != nil {
		return nil, err
	}

	return &namespacedDB{
		DB:            testDB,
		parentRawConn: db,
		namespace:     namespacedTestDB,
		autoDrop:      true,
	}, nil
}

// CockroachDefined returns an error when no database connection string is provided
func CockroachDefined() error {
	if *pgtest.CrdbConnStr == "" {
		return errs.New("flag --cockroach-test-db or environment variable STORJ_COCKROACH_TEST not defined for CockroachDB test database")
	}
	return nil
}

// namespacedDB implements namespacing for new satellite.DB databases when testing
type namespacedDB struct {
	satellite.DB

	parentRawConn *dbx.DB
	namespace     string
	autoDrop      bool
}

// Close closes the namespaced test database. If autoDrop is true,
// then we make a database connection to the parent db and delete the
// namespaced database that was used for testing.
func (db *namespacedDB) Close() error {
	err := db.DB.Close()
	if err != nil {
		return err
	}

	var dropErr error
	if db.autoDrop {
		// connect to the parent db and delete the namespaced database used for the test
		_, dropErr = db.parentRawConn.Exec(fmt.Sprintf("DROP DATABASE %s;", pq.QuoteIdentifier(db.namespace)))
	}

	return errs.Combine(dropErr, db.parentRawConn.Close())
}

// CreateTables creates table for the namespaced test database
func (db *namespacedDB) CreateTables() error {
	return db.DB.CreateTables()
}
