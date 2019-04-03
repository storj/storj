// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"database/sql"

	"github.com/zeebo/errs"
)

// DB is the minimal implementation that is needed by migration.
type DB interface {
	Begin() (*sql.Tx, error)
	Schema() string
	Rebind(string) string
}

// Error is the default migrate errs class
var Error = errs.Class("migrate")

// Create with a previous schema check
func Create(identifier string, db DB) error {
	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	schema := db.Schema()

	_, err = tx.Exec(db.Rebind(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text);`))
	if err != nil {
		return Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	row := tx.QueryRow(db.Rebind(`SELECT schemaText FROM table_schemas WHERE id = ?;`), identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if err == sql.ErrNoRows {
		_, err := tx.Exec(schema)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		_, err = tx.Exec(db.Rebind(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?);`), identifier, schema)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}

		return Error.Wrap(tx.Commit())
	}
	if err != nil {
		return Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	if schema != previousSchema {
		err := Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, schema)
		return Error.Wrap(errs.Combine(err, tx.Rollback()))
	}

	return Error.Wrap(tx.Rollback())
}
