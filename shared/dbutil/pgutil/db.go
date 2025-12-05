// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbschema"
	"storj.io/storj/shared/tagsql"
)

var (
	mon = monkit.Package()
)

// OpenUnique opens a postgres database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/shared/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connstr string, schemaPrefix string) (*dbutil.TempDatabase, error) {
	// sanity check, because you get an unhelpful error message when this happens
	if strings.HasPrefix(connstr, "cockroach://") {
		return nil, errs.New("can't connect to cockroach using pgutil.OpenUnique()! connstr=%q. try tempdb.OpenUnique() instead?", connstr)
	}

	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)
	connStrWithSchema := ConnstrWithSchema(connstr, schemaName)

	db, err := tagsql.Open(ctx, "pgx", connStrWithSchema, nil)
	if err == nil {
		// check that connection actually worked before trying CreateSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		return nil, errs.New("failed to connect to %q with driver pgx: %w", connStrWithSchema, err)
	}

	err = CreateSchema(ctx, db, schemaName)
	if err != nil {
		return nil, errs.Combine(err, db.Close())
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		childCtx, cancel := context.WithTimeout(context2.WithoutCancellation(ctx), 15*time.Second)
		defer cancel()
		return DropSchema(childCtx, cleanupDB, schemaName)
	}

	dbutil.Configure(ctx, db, "tmp_postgres", mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        connStrWithSchema,
		Schema:         schemaName,
		Driver:         "pgx",
		Implementation: dbutil.Postgres,
		Cleanup:        cleanup,
	}, nil
}

// QuerySnapshot loads snapshot from database.
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

// EnsureApplicationName ensures that the Connection String contains an application name.
func EnsureApplicationName(s string, app string) (string, error) {
	if !strings.Contains(s, "application_name") {
		if strings.TrimSpace(app) == "" {
			return s, errs.New("application name cannot be empty")
		}

		if !strings.Contains(s, "?") {
			return s + "?application_name=" + app, nil
		}

		return s + "&application_name=" + app, nil
	}
	// return source as is if application_name is set
	return s, nil
}

// QuoteIdentifier quotes an identifier for use in an interpolated SQL string.
func QuoteIdentifier(ident string) string {
	return pgx.Identifier{ident}.Sanitize()
}

// UnquoteIdentifier is the analog of QuoteIdentifier.
func UnquoteIdentifier(quotedIdent string) string {
	if len(quotedIdent) >= 2 && quotedIdent[0] == '"' && quotedIdent[len(quotedIdent)-1] == '"' {
		quotedIdent = strings.ReplaceAll(quotedIdent[1:len(quotedIdent)-1], "\"\"", "\"")
	}
	return quotedIdent
}
