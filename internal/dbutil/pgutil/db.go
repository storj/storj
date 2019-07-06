// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"database/sql"
	"strings"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/dbutil/dbschema"
)

// DB is postgres database with schema
type DB struct {
	*sql.DB
	Schema string
}

var (
	mon = monkit.Package()
)

// Open opens a postgres database with a schema
func Open(connstr string, schemaPrefix string) (*DB, error) {
	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)

	db, err := sql.Open("postgres", ConnstrWithSchema(connstr, schemaName))
	if err != nil {
		return nil, err
	}

	dbutil.Configure(db, mon)

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
	if !strings.Contains(s, "application_name") {
		if !strings.Contains(s, "?") {
			r = s + "?application_name=Satellite"
			return
		}
		r = s + "&application_name=Satellite"
		return
	}
	//return source as is if application_name is set
	return s
}

// IsConstraintError checks if given error is about constraint violation
func IsConstraintError(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		if e, ok := err.(*pq.Error); ok {
			if e.Code.Class() == "23" {
				return true
			}
		}
		return false
	})
}
