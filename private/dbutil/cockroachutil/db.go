// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
)

var mon = monkit.Package()

// CreateRandomTestingSchemaName creates a random schema name string.
func CreateRandomTestingSchemaName(n int) string {
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}

// OpenUnique opens a temporary unique CockroachDB database that will be cleaned up when closed.
// It is expected that this should normally be used by way of
// "storj.io/storj/private/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connStr string, schemaPrefix string) (db *dbutil.TempDatabase, err error) {
	if !strings.HasPrefix(connStr, "cockroach://") {
		return nil, errs.New("expected a cockroachDB URI, but got %q", connStr)
	}

	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)

	masterDB, err := tagsql.Open(ctx, "cockroach", connStr)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, masterDB.Close())
	}()

	err = masterDB.PingContext(ctx)
	if err != nil {
		return nil, errs.New("Could not open masterDB at conn %q: %w", connStr, err)
	}

	_, err = masterDB.Exec(ctx, "CREATE DATABASE "+pgutil.QuoteIdentifier(schemaName))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		_, err := cleanupDB.Exec(context.TODO(), "DROP DATABASE "+pgutil.QuoteIdentifier(schemaName))
		return errs.Wrap(err)
	}

	modifiedConnStr, err := changeDBTargetInConnStr(connStr, schemaName)
	if err != nil {
		return nil, errs.Combine(err, cleanup(masterDB))
	}

	sqlDB, err := tagsql.Open(ctx, "cockroach", modifiedConnStr)
	if err != nil {
		return nil, errs.Combine(errs.Wrap(err), cleanup(masterDB))
	}

	dbutil.Configure(ctx, sqlDB, "tmp_cockroach", mon)
	return &dbutil.TempDatabase{
		DB:             sqlDB,
		ConnStr:        modifiedConnStr,
		Schema:         schemaName,
		Driver:         "cockroach",
		Implementation: dbutil.Cockroach,
		Cleanup:        cleanup,
	}, nil
}

func changeDBTargetInConnStr(connStr string, newDBName string) (string, error) {
	connURL, err := url.Parse(connStr)
	if err != nil {
		return "", errs.Wrap(err)
	}
	connURL.Path = newDBName
	return connURL.String(), nil
}
