// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	_ "github.com/googleapis/go-sql-spanner" // register the spanner driver
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

// CreateRandomTestingDatabaseName creates a random schema name string.
func CreateRandomTestingDatabaseName(n int) string {
	// hex will increase the encoded length by 2 as documented by hex.EncodedLen()
	n /= 2
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}

// OpenUnique opens a spanner database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/shared/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connstr string, databasePrefix string) (*dbutil.TempDatabase, error) {
	params, err := ParseConnStr(connstr)
	if err != nil {
		return nil, errs.New("failed to parse spanner connection string %s: %w", connstr, err)
	}

	// spanner database names are between 2-30 characters
	numRandomCharacters := 4
	possibleRandomCharacters := 30 - len(databasePrefix) - 1
	if numRandomCharacters < possibleRandomCharacters {
		numRandomCharacters = possibleRandomCharacters
	}
	schemaName := databasePrefix + "_" + CreateRandomTestingDatabaseName(numRandomCharacters)

	// TODO(spanner): should we allow hardcoding a database name for testing with production spanner?
	params.Database = EscapeCharacters(schemaName)
	err = CreateDatabase(ctx, params)
	if err != nil {
		return nil, errs.New("failed to create database in spanner: %w", err)
	}

	db, err := tagsql.Open(ctx, "spanner", params.GoSqlSpannerConnStr())
	if err == nil {
		// check that connection actually worked before trying createSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		return nil, errs.New("failed to connect to %q with driver spanner: %w", connstr, err)
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		childCtx, cancel := context2.WithRetimeout(ctx, 15*time.Second)
		defer cancel()
		return dropDatabase(childCtx, params)
	}

	dbutil.Configure(ctx, db, "tmp_spanner", mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        params.ConnStr(),
		Schema:         schemaName,
		Driver:         "spanner",
		Implementation: dbutil.Spanner,
		Cleanup:        cleanup,
	}, nil
}

// CreateDatabase creates a schema in spanner with the given name.
func CreateDatabase(ctx context.Context, params ConnParams) error {
	admin, err := database.NewDatabaseAdminClient(ctx, params.ClientOptions()...)
	if err != nil {
		return fmt.Errorf("failed to create database admin: %w", err)
	}

	ddl, err := admin.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          params.InstancePath(),
		DatabaseDialect: databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
		CreateStatement: "CREATE DATABASE `" + params.Database + "`",
		ExtraStatements: []string{},
	})
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if _, err := ddl.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait database creation: %w", err)
	}
	if err := admin.Close(); err != nil {
		return fmt.Errorf("failed to close database admin: %w", err)
	}
	return nil
}

func dropDatabase(ctx context.Context, params ConnParams) error {
	admin, err := database.NewDatabaseAdminClient(ctx, params.ClientOptions()...)
	if err != nil {
		return fmt.Errorf("failed to create database admin: %w", err)
	}

	if err := admin.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: params.DatabasePath()}); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if err := admin.Close(); err != nil {
		return fmt.Errorf("failed to close database admin: %w", err)
	}
	return nil
}

// EscapeCharacters escapes non-spanner name compatible characters.
func EscapeCharacters(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "`", "\\`")
}
