// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/testcontext"
)

func TestDatabase(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	t.Run("BeginTx return err when db is nil", func(t *testing.T) {
		db := &Database{
			db: nil,
		}

		transaction, err := db.BeginTx(ctx)
		assert.NotNil(t, err)
		assert.Error(t, err)
		assert.Nil(t, transaction)
	})

	t.Run("BeginTx and Commit success", func(t *testing.T) {
		db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}

		transaction, err := db.BeginTx(nil)
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.NotNil(t, transaction)

		err = transaction.Commit()
		assert.Nil(t, err)
		assert.NoError(t, err)

		err = db.Close()
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("BeginTx and Rollback success", func(t *testing.T) {
		db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}

		transaction, err := db.BeginTx(nil)
		assert.Nil(t, err)
		assert.NoError(t, err)
		assert.NotNil(t, transaction)

		err = transaction.Rollback()
		assert.Nil(t, err)
		assert.NoError(t, err)

		err = db.Close()
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Commit fails", func(t *testing.T) {
		transaction := &DBTx{
			Database: &Database{
				tx: nil,
			},
		}

		err := transaction.Commit()
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Rollback fails", func(t *testing.T) {
		transaction := &DBTx{
			Database: &Database{
				tx: nil,
			},
		}

		err := transaction.Rollback()
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
