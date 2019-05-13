// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate go-bindata -o data.go -pkg schema -ignore ".*go" .

package schema

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v3"
	"github.com/golang-migrate/migrate/v3/database/postgres"
	bindata "github.com/golang-migrate/migrate/v3/source/go_bindata"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/pgutil"
)

// PrepareDB applies schema migrations as necessary to the given database to
// get it up to date.
func PrepareDB(db *sql.DB, dbURL string) error {
	srcDriver, err := bindata.WithInstance(bindata.Resource(AssetNames(),
		func(name string) ([]byte, error) {
			return Asset(name)
		}))
	if err != nil {
		return err
	}

	schema, err := pgutil.ParseSchemaFromConnstr(dbURL)
	if err != nil {
		return errs.New("error parsing schema: %+v", err)
	}
	if schema != "" {
		err := pgutil.CreateSchema(db, schema)
		if err != nil {
			return errs.New("error creating schema: %+v", err)
		}
	}

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("go-bindata migrations", srcDriver, "postgreskv db", dbDriver)
	if err != nil {
		return err
	}
	err = m.Up()
	if err == migrate.ErrNoChange {
		err = nil
	}
	return err
}
