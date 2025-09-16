// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"encoding/hex"
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

		t.Run("UpsertBatch", func(t *testing.T) {
			require.NoError(t, tailsDB.UpsertBatch(ctx, nil))
			require.NoError(t, tailsDB.UpsertBatch(ctx, []console.APIKeyTail{}))

			batch := []console.APIKeyTail{*tail}
			require.NoError(t, tailsDB.UpsertBatch(ctx, batch))
			dbTail, err := tailsDB.GetByTail(ctx, tail.Tail)
			require.NoError(t, err)
			require.EqualValues(t, tail.Tail, dbTail.Tail)

			updated := later.Add(3 * time.Minute)
			batch[0].LastUsed = updated
			require.NoError(t, tailsDB.UpsertBatch(ctx, batch))
			dbTail, err = tailsDB.GetByTail(ctx, tail.Tail)
			require.NoError(t, err)
			require.WithinDuration(t, updated, dbTail.LastUsed, time.Minute)

			secondTail := console.APIKeyTail{
				RootKeyID:  info.ID,
				Tail:       testrand.Bytes(5 * memory.B),
				ParentTail: testrand.Bytes(3 * memory.B),
				Caveat:     testrand.Bytes(7 * memory.B),
				LastUsed:   now,
			}
			multi := []console.APIKeyTail{*tail, secondTail}
			require.NoError(t, tailsDB.UpsertBatch(ctx, multi))

			dbTail1, err := tailsDB.GetByTail(ctx, tail.Tail)
			require.NoError(t, err)
			require.EqualValues(t, tail.Tail, dbTail1.Tail)
			dbTail2, err := tailsDB.GetByTail(ctx, secondTail.Tail)
			require.NoError(t, err)
			require.EqualValues(t, secondTail.Tail, dbTail2.Tail)
			require.WithinDuration(t, now, dbTail2.LastUsed, time.Minute)

			updated = later.Add(10 * time.Minute)
			multi[0].LastUsed = updated
			multi[1].LastUsed = updated
			require.NoError(t, tailsDB.UpsertBatch(ctx, multi))

			dbTail1, err = tailsDB.GetByTail(ctx, tail.Tail)
			require.NoError(t, err)
			require.WithinDuration(t, updated, dbTail1.LastUsed, time.Minute)
			dbTail2, err = tailsDB.GetByTail(ctx, secondTail.Tail)
			require.NoError(t, err)
			require.WithinDuration(t, updated, dbTail2.LastUsed, time.Minute)
		})
	})
}

func TestAPIKeyTails_checkExistenceBatch(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		tailsDB := db.Console().APIKeyTails()
		apiKeysDB := db.Console().APIKeys()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@example.test",
			PasswordHash: testrand.Bytes(8),
		})
		require.NoError(t, err)

		project, err := db.Console().Projects().Insert(ctx, &console.Project{
			ID:      testrand.UUID(),
			Name:    "Test Project",
			OwnerID: user.ID,
		})
		require.NoError(t, err)

		secret, err := macaroon.NewSecret()
		require.NoError(t, err)
		apiKey, err := macaroon.NewAPIKey(secret)
		require.NoError(t, err)

		keyInfo, err := apiKeysDB.Create(ctx, apiKey.Head(), console.APIKeyInfo{
			ProjectID: project.ID,
			Name:      "test key",
			Secret:    secret,
		})
		require.NoError(t, err)

		tail1 := testrand.Bytes(32)
		tail2 := testrand.Bytes(32)
		nonExistentTail := testrand.Bytes(32)

		testTail1 := &console.APIKeyTail{
			RootKeyID:  keyInfo.ID,
			Tail:       tail1,
			ParentTail: testrand.Bytes(32),
			Caveat:     testrand.Bytes(32),
			LastUsed:   time.Now(),
		}
		testTail2 := &console.APIKeyTail{
			RootKeyID:  keyInfo.ID,
			Tail:       tail2,
			ParentTail: testrand.Bytes(32),
			Caveat:     testrand.Bytes(32),
			LastUsed:   time.Now(),
		}

		_, err = tailsDB.Upsert(ctx, testTail1)
		require.NoError(t, err)
		_, err = tailsDB.Upsert(ctx, testTail2)
		require.NoError(t, err)

		t.Run("batch existence check", func(t *testing.T) {
			results, err := tailsDB.CheckExistenceBatch(ctx, [][]byte{tail1, tail2, nonExistentTail})
			require.NoError(t, err)
			require.Len(t, results, 3)
			require.True(t, results[hex.EncodeToString(tail1)])
			require.True(t, results[hex.EncodeToString(tail2)])
			require.False(t, results[hex.EncodeToString(nonExistentTail)])
		})

		t.Run("empty batch", func(t *testing.T) {
			results, err := tailsDB.CheckExistenceBatch(ctx, [][]byte{})
			require.NoError(t, err)
			require.Empty(t, results)
		})

		t.Run("single tail batch", func(t *testing.T) {
			results, err := tailsDB.CheckExistenceBatch(ctx, [][]byte{tail1})
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.True(t, results[hex.EncodeToString(tail1)])
		})
	})
}
