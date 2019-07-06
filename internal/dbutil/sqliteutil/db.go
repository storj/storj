// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"database/sql"
	"strconv"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/dbschema"
)

// LoadSchemaFromSQL inserts script into connstr and loads schema.
func LoadSchemaFromSQL(script string) (_ *dbschema.Schema, err error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.Exec(script)
	if err != nil {
		return nil, err
	}

	return QuerySchema(db)
}

// LoadSnapshotFromSQL inserts script into connstr and loads schema.
func LoadSnapshotFromSQL(script string) (_ *dbschema.Snapshot, err error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.Exec(script)
	if err != nil {
		return nil, err
	}

	snapshot, err := QuerySnapshot(db)
	if err != nil {
		return nil, err
	}

	snapshot.Script = script
	return snapshot, nil
}

// QuerySnapshot loads snapshot from database
func QuerySnapshot(db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(db)
	if err != nil {
		return nil, err
	}

	data, err := QueryData(db, schema)
	if err != nil {
		return nil, err
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, err
}

// QueryData loads all data from tables
func QueryData(db dbschema.Queryer, schema *dbschema.Schema) (*dbschema.Data, error) {
	return dbschema.QueryData(db, schema, func(columnName string) string {
		quoted := strconv.Quote(columnName)
		return `quote(` + quoted + `) as ` + quoted
	})
}

// IsConstraintError checks if given error is about constraint violation
func IsConstraintError(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		if e, ok := err.(sqlite3.Error); ok {
			if e.Code == sqlite3.ErrConstraint {
				return true
			}
		}
		return false
	})
}
