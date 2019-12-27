// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestConsoleTx(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		console := db.Console()

		t.Run("BeginTx, Commit, Rollback", func(t *testing.T) {
			tx, err := console.BeginTx(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, tx)

			// TODO: add something into database

			assert.NoError(t, tx.Commit())
			assert.Error(t, tx.Rollback())

			// TODO: check whether it has been committed
		})

		t.Run("BeginTx, Rollback, Commit", func(t *testing.T) {
			tx, err := console.BeginTx(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, tx)

			// TODO: add something into database

			assert.NoError(t, tx.Rollback())
			assert.Error(t, tx.Commit())

			// TODO: check whether it has been rolled back
		})
	})
}
