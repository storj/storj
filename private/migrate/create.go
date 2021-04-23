// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/private/dbutil/txutil"
	"storj.io/private/tagsql"
)

// Error is the default migrate errs class.
var Error = errs.Class("migrate")

// Create with a previous schema check.
func Create(ctx context.Context, identifier string, db DBX) error {
	// is this necessary? it's not immediately obvious why we roll back the transaction
	// when the schemas match.
	justRollbackPlease := errs.Class("only used to tell WithTx to do a rollback")

	err := txutil.WithTx(ctx, db, nil, func(ctx context.Context, tx tagsql.Tx) (err error) {
		schema := db.Schema()

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
