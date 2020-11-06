// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package apikey_test

import (
	"testing"
	"time"

	"github.com/zeebo/assert"

	"storj.io/common/testcontext"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikey"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestSecretDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		secrets := db.Secret()
		token, err := apikey.NewSecretToken()
		assert.NoError(t, err)
		token2, err := apikey.NewSecretToken()
		assert.NoError(t, err)

		t.Run("Test StoreSecret", func(t *testing.T) {
			err := secrets.Store(ctx, apikey.APIKey{
				Secret:    token,
				CreatedAt: time.Now().UTC(),
			})
			assert.NoError(t, err)
		})

		t.Run("Test CheckSecret", func(t *testing.T) {
			err := secrets.Check(ctx, token)
			assert.NoError(t, err)

			err = secrets.Check(ctx, token2)
			assert.Error(t, err)
		})

		t.Run("Test RevokeSecret", func(t *testing.T) {
			err = secrets.Revoke(ctx, token)
			assert.NoError(t, err)

			err = secrets.Check(ctx, token)
			assert.Error(t, err)
		})

	})
}
