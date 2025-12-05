// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqliteutil

import (
	"context"
	"strconv"
	"strings"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/tagsql"
)

// LoadSchemaFromSQL inserts script into connstr and loads schema.
func LoadSchemaFromSQL(ctx context.Context, script []string) (_ *dbschema.Schema, err error) {
	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.ExecContext(ctx, strings.Join(script, ";\n"))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return QuerySchema(ctx, db)
}

// LoadSnapshotFromSQL inserts script into connstr and loads schema.
func LoadSnapshotFromSQL(ctx context.Context, script string) (_ *dbschema.Snapshot, err error) {
	db, err := tagsql.Open(ctx, "sqlite3", ":memory:", nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	_, err = db.ExecContext(ctx, script)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	snapshot, err := QuerySnapshot(ctx, db)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	snapshot.Sections = dbschema.NewSections(script)

	return snapshot, nil
}

// QuerySnapshot loads snapshot from database.
func QuerySnapshot(ctx context.Context, db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(ctx, db)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	data, err := QueryData(ctx, db, schema)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, err
}

// QueryData loads all data from tables.
func QueryData(ctx context.Context, db dbschema.Queryer, schema *dbschema.Schema) (*dbschema.Data, error) {
	return dbschema.QueryData(ctx, db, schema, func(columnName string) string {
		quoted := strconv.Quote(columnName)
		return `quote(` + quoted + `) as ` + quoted
	})
}

// IsConstraintError checks if given error is about constraint violation.
func IsConstraintError(err error) bool {
	return errs.IsFunc(err, func(err error) bool {
		if e, ok := err.(sqlite3.Error); ok { //nolint: errorlint // IsFunc implements the unwrap loop.
			if e.Code == sqlite3.ErrConstraint {
				return true
			}
		}
		return false
	})
}
