// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package dbcleanup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/macaroon"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/dbcleanup"
)

func TestChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		cfg := sat.Config

		user1, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "example1@mail.test",
		}, 1)
		require.NoError(t, err)

		user2, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "example2@mail.test",
		}, 1)
		require.NoError(t, err)

		pr1, err := sat.AddProject(ctx, user1.ID, "Test Project")
		require.NoError(t, err)
		pr2, err := sat.AddProject(ctx, user2.ID, "Test Project")
		require.NoError(t, err)

		chore := dbcleanup.NewChore(zaptest.NewLogger(t), db.Console(), cfg.ConsoleDBCleanup, cfg.Console.Config)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		t.Run("delete expired keys", func(t *testing.T) {
			chore.Loop.Pause()

			secret, err := macaroon.NewSecret()
			require.NoError(t, err)

			key1, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)
			key2, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)
			key3, err := macaroon.NewAPIKey(secret)
			require.NoError(t, err)

			now := time.Now()

			keyInfo1 := console.APIKeyInfo{
				Name:      cfg.Console.ObjectBrowserKeyNamePrefix,
				ProjectID: pr1.ID,
				Secret:    secret,
			}
			keyInfo2 := console.APIKeyInfo{
				Name:      cfg.Console.ObjectBrowserKeyNamePrefix,
				ProjectID: pr2.ID,
				Secret:    secret,
			}
			keyInfo3 := console.APIKeyInfo{
				Name:      "randomName",
				ProjectID: pr2.ID,
				Secret:    secret,
			}

			createdKey1, err := db.Console().APIKeys().Create(ctx, key1.Head(), keyInfo1)
			require.NoError(t, err)
			require.NotNil(t, createdKey1)
			createdKey2, err := db.Console().APIKeys().Create(ctx, key2.Head(), keyInfo2)
			require.NoError(t, err)
			require.NotNil(t, createdKey2)
			createdKey3, err := db.Console().APIKeys().Create(ctx, key3.Head(), keyInfo3)
			require.NoError(t, err)
			require.NotNil(t, createdKey3)

			query := db.Testing().Rebind("UPDATE api_keys SET created_at = ? WHERE id = ?")
			createdAt := now.Add(-cfg.Console.ObjectBrowserKeyLifetime).Add(-time.Hour)

			_, err = db.Testing().RawDB().ExecContext(ctx, query, createdAt, createdKey1.ID)
			require.NoError(t, err)
			_, err = db.Testing().RawDB().ExecContext(ctx, query, createdAt, createdKey3.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// Expired key is removed.
			createdKey1, err = db.Console().APIKeys().Get(ctx, createdKey1.ID)
			require.Error(t, err)
			require.Nil(t, createdKey1)

			// Non-expired key and expired but not-prefixed key are both present.
			cursor := console.APIKeyCursor{Page: 1, Limit: 10}

			page, err := db.Console().APIKeys().GetPagedByProjectID(ctx, pr2.ID, cursor, "")
			require.NoError(t, err)
			require.NotNil(t, page)
			require.Len(t, page.APIKeys, 2)

			_, err = db.Testing().RawDB().ExecContext(ctx, query, createdAt, createdKey2.ID)
			require.NoError(t, err)

			chore.Loop.TriggerWait()

			// Second expired key is removed.
			createdKey2, err = db.Console().APIKeys().Get(ctx, createdKey2.ID)
			require.Error(t, err)
			require.Nil(t, createdKey2)
		})
	})
}
