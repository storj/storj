// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"

	"cloud.google.com/go/spanner"
	sqlspanner "github.com/googleapis/go-sql-spanner"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"google.golang.org/api/option"

	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/dbschema"
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

var rxInsertQuery = regexp.MustCompile(`(?i)^\s*insert\s+into\b`)

// OpenUnique opens a spanner database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/shared/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, log *zap.Logger, connstr string, databasePrefix string, extraStatements []string) (*dbutil.TempDatabase, error) {
	// separate DDL and DML queries
	// TODO(spanner): this only handles insert queries, which is sufficient for tests, but it's not a general solution.
	insertStatements := []string{}
	extraStatements = slices.DeleteFunc(extraStatements, func(v string) bool {
		if rxInsertQuery.MatchString(v) {
			insertStatements = append(insertStatements, v)
			return true
		}
		return false
	})

	ephemeral, err := CreateEphemeralDB(ctx, connstr, databasePrefix, extraStatements...)
	if err != nil {
		return nil, errs.New("failed to create database: %w", err)
	}

	connectorConfig, err := sqlspanner.ExtractConnectorConfig(ephemeral.Params.GoSqlSpannerConnStr())
	if err != nil {
		return nil, errs.New("failed to parse connector config: %w", err)
	}

	connectorConfig.Configurator = func(config *spanner.ClientConfig, opts *[]option.ClientOption) {
		config.Logger = zap.NewStdLog(log)
		if ephemeral.Params.Emulator {
			config.SessionPoolConfig.MinOpened = 100
		}
	}

	connector, err := sqlspanner.CreateConnector(connectorConfig)
	if err != nil {
		return nil, errs.New("failed to create connector: %w", err)
	}
	db := tagsql.Wrap(sql.OpenDB(connector))
	if err == nil {
		// check that connection actually worked before trying createSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		_ = ephemeral.Close(ctx)
		return nil, errs.New("failed to connect to %q with driver spanner: %w", connstr, err)
	}

	for _, query := range insertStatements {
		_, err := db.ExecContext(ctx, query)
		if err != nil {
			_ = ephemeral.Close(ctx)
			return nil, errs.New("failed to execute %q with driver spanner: %w", query, err)
		}
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

// QuerySnapshot loads snapshot from database.
func QuerySnapshot(ctx context.Context, db dbschema.Queryer) (*dbschema.Snapshot, error) {
	schema, err := QuerySchema(ctx, db)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	data, err := QueryData(ctx, db, schema)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &dbschema.Snapshot{
		Version: -1,
		Schema:  schema,
		Data:    data,
	}, err
}

// MultiExecDBWrapper wraps a tagsql.DB to override ExecContext behavior;
// namely, it splits up queries containing multiple statements and executes
// them individually.
//
// This must only be used in cases where it is acceptable for some statements
// to succeed and others to fail. There is (currently) no way to get
// transactional behavior for multiple DDL statements in Spanner.
type MultiExecDBWrapper struct {
	tagsql.DB
}

// ExecContext executes all statements in a query, separated by semicolons.
// Important: the result returned is that of the _last_ statement, not any
// sort of combination of all results.
func (m *MultiExecDBWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (result sql.Result, err error) {
	queries, err := SplitSQLStatements(query)
	if err != nil {
		return nil, err
	}
	for i, q := range queries {
		result, err = m.DB.ExecContext(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("in SQL statement #%d: %+w", i+1, err)
		}
	}
	return result, nil
}
