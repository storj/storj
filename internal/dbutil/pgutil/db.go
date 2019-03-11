// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"database/sql"
	"fmt"
	"strings"

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

// LoadSnapshotFromSQL inserts script into connstr and loads schema.
func LoadSnapshotFromSQL(connstr, script string) (_ *dbschema.Snapshot, err error) {
	db, err := Open(connstr, "load-schema")
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

//CheckApplicationName ensures that the Connection String contains an application name
func CheckApplicationName(s string) (r string) {
	if !strings.Contains(s, "ApplicationName") {
		if !strings.Contains(s, "?") {
			r = fmt.Sprintf("%s?%s", s, "application_name=Satellite")
		} else {
			r = fmt.Sprintf("%s%s%s", s, "&", "application_name=Satellite")
		}
		return
	}
	//return source as is if ApplicationName is set
	return s
}
