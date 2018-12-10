// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"flag"
	"os"
)

const (
	// postgres connstring that works with docker-compose
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
	defaultSqliteConn   = "sqlite3://file::memory:?mode=memory&cache=shared"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

type basicLogger interface {
	Log(args ...interface{})
	Fatal(args ...interface{})
}

// ForEach method will iterate over all supported databases. Will establish
// connection and will create tables for each DB.
func ForEach(logger basicLogger, test func(db *DB)) {
	for _, dbInfo := range []struct {
		dbName    string
		dbURL     string
		dbMessage string
	}{
		{"Sqlite", defaultSqliteConn, ""},
		{"Postgres", *testPostgres, "Postgres flag missing, example: -postgres-test-db=" + defaultPostgresConn},
	} {
		if dbInfo.dbURL == "" {
			logger.Log("Database", dbInfo.dbName, "connection string not provided.", dbInfo.dbMessage)
			continue
		}

		logger.Log("Start testing", dbInfo.dbName)

		db, err := NewDB(dbInfo.dbURL)
		if err != nil {
			logger.Fatal(err)
		}

		defer func() {
			err := db.Close()
			if err != nil {
				logger.Fatal(err)
			}
		}()

		err = db.CreateTables()
		if err != nil {
			logger.Fatal(err)
		}

		test(db)

		logger.Log("Stop testing", dbInfo.dbName)
	}
}
