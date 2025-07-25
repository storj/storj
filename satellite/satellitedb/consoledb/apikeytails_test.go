// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestAPIKeyTails(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		now := time.Now()
		later := now.Add(5 * time.Minute)

		pr, err := db.Console().Projects().Insert(ctx, &console.Project{Name: "test-project"})
		require.NoError(t, err)

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)
		apiKey, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		info, err := db.Console().APIKeys().Create(ctx, apiKey.Head(), console.APIKeyInfo{
			ProjectID: pr.ID,
			Name:      "test-key",
			Secret:    secret,
			Version:   macaroon.APIKeyVersionAuditable,
		})
		require.NoError(t, err)

		tail := &console.APIKeyTail{
			RootKeyID:  info.ID,
			Tail:       testrand.Bytes(2 * memory.B),
			ParentTail: testrand.Bytes(3 * memory.B),
			Caveat:     testrand.Bytes(4 * memory.B),
			LastUsed:   now,
		}

		tailsDB := db.Console().APIKeyTails()

		t.Run("Can't insert nil tail", func(t *testing.T) {
			dbTail, err := tailsDB.Upsert(ctx, nil)
			require.Error(t, err)
			require.Nil(t, dbTail)
		})

		t.Run("Upsert tail", func(t *testing.T) {
			dbTail, err := tailsDB.Upsert(ctx, tail)
			require.NoError(t, err)
			require.NotNil(t, dbTail)
			require.EqualValues(t, info.ID, dbTail.RootKeyID)
			require.EqualValues(t, tail.Tail, dbTail.Tail)
			require.EqualValues(t, tail.ParentTail, dbTail.ParentTail)
			require.EqualValues(t, tail.Caveat, dbTail.Caveat)
			require.WithinDuration(t, now, dbTail.LastUsed, time.Minute)

			tail.LastUsed = later

			dbTail, err = tailsDB.Upsert(ctx, tail)
			require.NoError(t, err)
			require.NotNil(t, dbTail)
			require.EqualValues(t, info.ID, dbTail.RootKeyID)
			require.EqualValues(t, tail.Tail, dbTail.Tail)
			require.EqualValues(t, tail.ParentTail, dbTail.ParentTail)
			require.EqualValues(t, tail.Caveat, dbTail.Caveat)
			require.WithinDuration(t, later, dbTail.LastUsed, time.Minute)
		})

		t.Run("Get tail", func(t *testing.T) {
			dbTail, err := tailsDB.GetByTail(ctx, tail.Tail)
			require.NoError(t, err)
			require.NotNil(t, dbTail)
			require.EqualValues(t, info.ID, dbTail.RootKeyID)
			require.EqualValues(t, tail.Tail, dbTail.Tail)
			require.EqualValues(t, tail.ParentTail, dbTail.ParentTail)
			require.EqualValues(t, tail.Caveat, dbTail.Caveat)
			require.WithinDuration(t, later, dbTail.LastUsed, time.Minute)
		})
	})
}
