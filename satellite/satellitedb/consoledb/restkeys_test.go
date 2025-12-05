// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRestApiKeys(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		users := db.Console().Users()
		apiKeys := db.Console().RestApiKeys()

		userID, err := uuid.New()
		require.NoError(t, err)

		user, err := users.Insert(ctx, &console.User{
			ID:           userID,
			Email:        "some@test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)

		userID = user.ID

		start := time.Now()

		pointerTime := func(t time.Time) *time.Time {
			return &t
		}

		allKeys := []restapikeys.Key{
			{
				UserID:    userID,
				Token:     "expired",
				CreatedAt: start.Add(-2 * time.Hour),
				ExpiresAt: pointerTime(start.Add(-1 * time.Hour)),
			},
			{
				UserID:    userID,
				Token:     "valid",
				CreatedAt: start,
				ExpiresAt: pointerTime(start.Add(time.Hour)),
			},
			{
				UserID:    userID,
				Token:     "testToken",
				CreatedAt: start,
				ExpiresAt: pointerTime(start.Add(time.Hour)),
			},
		}
		var testCases []struct {
			ID    uuid.UUID
			token string
			err   error
		}
		for _, key := range allKeys {
			newKey, err := apiKeys.Create(ctx, key)
			require.NoError(t, err)

			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				err = sql.ErrNoRows
			}
			testCases = append(testCases, struct {
				ID    uuid.UUID
				token string
				err   error
			}{newKey.ID, newKey.Token, err})
		}

		for _, testCase := range testCases {
			_, err = apiKeys.GetByToken(ctx, testCase.token)
			require.Equal(t, testCase.err, err)

			_, err = apiKeys.Get(ctx, testCase.ID)
			require.Equal(t, testCase.err, err)

			err = apiKeys.Revoke(ctx, testCase.ID)
			require.NoError(t, err)

			_, err = apiKeys.Get(ctx, testCase.ID)
			require.ErrorIs(t, err, sql.ErrNoRows)

			err = apiKeys.Revoke(ctx, testCase.ID)
			require.ErrorIs(t, err, sql.ErrNoRows)
		}

		keys, err := apiKeys.GetAll(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, keys)

		_, err = apiKeys.Create(ctx, restapikeys.Key{
			UserID:    userID,
			Token:     "testToken",
			CreatedAt: start,
			ExpiresAt: pointerTime(start.Add(time.Hour)),
		})
		require.NoError(t, err)

		// create with duplicate token
		_, err = apiKeys.Create(ctx, restapikeys.Key{
			UserID:    userID,
			Token:     "testToken",
			CreatedAt: start,
			ExpiresAt: pointerTime(start.Add(time.Hour)),
		})
		require.Error(t, err)

		keys, err = apiKeys.GetAll(ctx, userID)
		require.NoError(t, err)
		require.Len(t, keys, 1)
	})
}
