// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/internal/dbutil/dbschema"
)

// DB is postgres database with schema
type DB struct {
	*sql.DB
	Schema string
}

// Open opens a postgres database with a schema
func Open(connstr string, schemaPrefix string) (*DB, error) {
	schemaName := schemaPrefix + "-" + RandomString(8)

	db, err := sql.Open("postgres", ConnstrWithSchema(connstr, schemaName))
	if err != nil {
		return nil, err
	}

	err = CreateSchema(db, schemaName)
	if err != nil {
		return nil, errs.Combine(err, db.Close())
	}

	return &DB{db, schemaName}, err
}

// Close closes the database and deletes the schema.
func (db *DB) Close() error {
	return errs.Combine(
		DropSchema(db.DB, db.Schema),
		db.DB.Close(),
	)
}

// LoadSchemaFromSQL inserts script into connstr and loads schema.
func LoadSchemaFromSQL(connstr, script string) (_ *dbschema.Schema, err error) {
	db, err := Open(connstr, "load-schema")
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

// LoadSchemaAndDataFromSQL inserts script into connstr and loads schema.
func LoadSchemaAndDataFromSQL(connstr, script string) (_ *dbschema.Schema, _ *dbschema.Data, err error) {
	db, err := Open(connstr, "load-schema")
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.Exec(script)
	if err != nil {
		return nil, nil, err
	}

	schema, err := QuerySchema(db)
	if err != nil {
		return nil, nil, err
	}

	data, err := QueryData(db, schema)
	if err != nil {
		return nil, nil, err
	}

	return schema, data, err
}
