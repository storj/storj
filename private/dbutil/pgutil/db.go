// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"context"
	"strings"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/dbschema"
	"storj.io/storj/private/tagsql"
)

var (
	mon = monkit.Package()
)

// OpenUnique opens a postgres database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/private/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connstr string, schemaPrefix string) (*dbutil.TempDatabase, error) {
	// sanity check, because you get an unhelpful error message when this happens
	if strings.HasPrefix(connstr, "cockroach://") {
		return nil, errs.New("can't connect to cockroach using pgutil.OpenUnique()! connstr=%q. try tempdb.OpenUnique() instead?", connstr)
	}

	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)
	connStrWithSchema := ConnstrWithSchema(connstr, schemaName)

	db, err := tagsql.Open("postgres", connStrWithSchema)
	if err == nil {
		// check that connection actually worked before trying CreateSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		return nil, errs.New("failed to connect to %q with driver postgres: %w", connStrWithSchema, err)
	}

	err = CreateSchema(ctx, db, schemaName)
	if err != nil {
		return nil, errs.Combine(err, db.Close())
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		return DropSchema(ctx, cleanupDB, schemaName)
	}

	dbutil.Configure(db, mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        connStrWithSchema,
		Schema:         schemaName,
		Driver:         "postgres",
		Implementation: dbutil.Postgres,
		Cleanup:        cleanup,
	}, nil
}

// QuerySnapshot loads snapshot from database
func QuerySnapshot(ctx context.Context, db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(ctx, db)
	if err != nil {
		return nil, err
	}

	data, err := QueryData(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, err
}

// CheckApplicationName ensures that the Connection String contains an application name
func CheckApplicationName(s string) (r string) {
	if !strings.Contains(s, "application_name") {
		if !strings.Contains(s, "?") {
			r = s + "?application_name=Satellite"
			return
		}
		r = s + "&application_name=Satellite"
		return
	}
	// return source as is if application_name is set
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
