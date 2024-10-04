// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	spannerdriver "github.com/googleapis/go-sql-spanner"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/txutil"
	"storj.io/storj/shared/tagsql"
)

// Error is the default migrate errs class.
var Error = errs.Class("migrate")

// Create with a previous schema check.
func Create(ctx context.Context, identifier string, db DBX) error {
	// is this necessary? it's not immediately obvious why we roll back the transaction
	// when the schemas match.
	justRollbackPlease := errs.Class("only used to tell WithTx to do a rollback")
	err := txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		schema := strings.Join(db.Schema(), ";\n")
		_, err = tx.ExecContext(ctx, db.Rebind(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text);`))
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, db.Rebind(`SELECT schemaText FROM table_schemas WHERE id = ?;`), identifier)
		var previousSchema string
		err = row.Scan(&previousSchema)
		// not created yet
		if errors.Is(err, sql.ErrNoRows) {
			_, err := tx.ExecContext(ctx, schema)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, db.Rebind(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?);`), identifier, schema)
			if err != nil {
				return err
			}
			return nil
		}
		if err != nil {
			return err
		}
		if schema != previousSchema {
			return Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, schema)
		}
		return justRollbackPlease.New("")
	})
	if justRollbackPlease.Has(err) {
		err = nil
	}
	return Error.Wrap(err)
}

// CreateSpanner creates the migration schema necessary to execute migrations in Spanner.
func CreateSpanner(ctx context.Context, identifier string, db DBX, forEmulator bool) error {
	schema := strings.Join(db.Schema(), ";\n")

	// Spanner does not support DDL in transactions https://github.com/googleapis/go-sql-spanner?tab=readme-ov-file#ddl-statements
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS table_schemas (id STRING(MAX), schemaText STRING(MAX)) PRIMARY KEY (id)`)
	if err != nil {
		return err
	}

	row := db.QueryRow(ctx, db.Rebind(`SELECT schemaText FROM table_schemas WHERE id = ?;`), identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if errors.Is(err, sql.ErrNoRows) {
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}()

		if err := conn.Raw(ctx, func(driverConn interface{}) error {
			// Get the Spanner connection interface and start a DDL batch on the connection.
			return driverConn.(spannerdriver.SpannerConn).StartBatchDDL()
		}); err != nil {
			return fmt.Errorf("conn.Raw failed: %w", err)
		}

		// TODO(spanner): workaround for bug in spanner emulator where
		// sequence names collide between databases.
		var uniqueSuffix string

		for _, schemaDDL := range db.Schema() {
			if forEmulator {
				if uniqueSuffix == "" {
					uniqueSuffix = generateUniqueSuffix()
				}

				if strings.Contains(schemaDDL, "SEQUENCE") {
					schemaDDL = rxSpannerCreateSequence.ReplaceAllStringFunc(schemaDDL, func(match string) string {
						return match + "_" + uniqueSuffix
					})
					schemaDDL = rxSpannerNextSequence.ReplaceAllStringFunc(schemaDDL, func(match string) string {
						return match + "_" + uniqueSuffix
					})
				}
			}

			if _, err := conn.ExecContext(ctx, schemaDDL); err != nil {
				return err
			}
		}

		if err := conn.Raw(ctx, func(driverConn interface{}) error {
			return driverConn.(spannerdriver.SpannerConn).RunBatch(ctx)
		}); err != nil {
			return Error.New("conn.Raw failed: %w", err)
		}

		if _, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?);`), identifier, schema); err != nil {
			return Error.Wrap(err)
		}

		return nil
	}
	if err != nil {
		return err
	}

	if schema != previousSchema {
		return Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, schema)
	}

	return Error.Wrap(err)
}

func generateUniqueSuffix() string {
	var uniqueSuffix [8]byte
	_, _ = rand.Read(uniqueSuffix[:])
	return hex.EncodeToString(uniqueSuffix[:])
}

var (
	rxSpannerCreateSequence = regexp.MustCompile(`CREATE SEQUENCE ([a-zA-Z_]+)`)
	rxSpannerNextSequence   = regexp.MustCompile(`GET_NEXT_SEQUENCE_VALUE\(SEQUENCE ([a-zA-Z_]+)`)
)
