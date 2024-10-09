// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	_ "github.com/googleapis/go-sql-spanner" // register the spanner driver
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

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
	ephemeral, err := CreateEphemeralDB(ctx, connstr, databasePrefix)
	if err != nil {
		return nil, errs.New("failed to create database: %w", err)
	}

	db, err := tagsql.Open(ctx, "spanner", ephemeral.Params.GoSqlSpannerConnStr())
	if err == nil {
		// check that connection actually worked before trying createSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		_ = ephemeral.Close(ctx)
		return nil, errs.New("failed to connect to %q with driver spanner: %w", connstr, err)
	}

	dbutil.Configure(ctx, db, "tmp_spanner", mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        ephemeral.Params.ConnStr(),
		Schema:         "",
		Driver:         "spanner",
		Implementation: dbutil.Spanner,
		Cleanup: func(cleanupDB tagsql.DB) error {
			// TODO: this ctx should be passed as a parameter to the cleanup func instead.
			return ephemeral.Close(ctx)
		},
	}, nil
}

// SplitDDL splits a multi-statement ddl into strings.
func SplitDDL(ddls string) []string {
	r := []string{}
	for _, ddl := range strings.Split(ddls, ";") {
		ddl = strings.TrimSpace(ddl)
		if ddl != "" {
			r = append(r, ddl)
		}
	}
	return r
}
