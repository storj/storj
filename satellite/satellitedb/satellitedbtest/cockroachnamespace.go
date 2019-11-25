// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbtest

import (
	"fmt"
	"regexp"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil/pgtest"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// NewCockroach creates a new satellite.DB that is used for testing. We create a new database
// with a unique name so that there aren'tt conflicts when we run tests. Postgres supports schemas
// for namespacing, but cockroachdb does not, so we are using a different database for each test instead
func NewCockroach(log *zap.Logger, namespacedTestDB string) (satellite.DB, error) {
	if err := DatabaseDefined(); err != nil {
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

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s;", namespacedTestDB))
	if err != nil {
		return nil, err
	}

	// match strings like this"/masterdb?"
	r, err := regexp.Compile("[/][a-zA-Z0-9]+[?]")
	if err != nil {
		return nil, err
	}
	testConnURL := r.ReplaceAllString(source, "/"+namespacedTestDB+"?")
	testDB, err := satellitedb.New(log, testConnURL)
	if err != nil {
		return nil, err
	}

	return &NamespacedDB{
		parentRawConn: db,
		DB:            testDB,
		Namespace:     namespacedTestDB,
		AutoDrop:      true,
	}, nil
}

// NamespacedDB implements namespacing via new databases for satellite.DB
type NamespacedDB struct {
	satellite.DB

	parentRawConn *dbx.DB
	Namespace     string
	AutoDrop      bool
}

// Close closes the database and drops the schema, when `AutoDrop` is set.
func (db *NamespacedDB) Close() error {
	err := db.DB.Close()
	if err != nil {
		return err
	}

	var dropErr error
	if db.AutoDrop {
		// connect to masterdb and delete database
		db.parentRawConn.Query(`drop database $1;`, db.Namespace)
	}

	return errs.Combine(dropErr, db.parentRawConn.Close())
}

// CreateTables creates the schema and creates tables.
func (db *NamespacedDB) CreateTables() error {
	return db.DB.CreateTables()
}
