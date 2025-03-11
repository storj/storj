// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package restkeys_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console/restkeys"
	"storj.io/storj/satellite/oidc"
)

func TestRESTKeys(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := restkeys.NewService(sat.DB.OIDC().OAuthTokens(), sat.Config.Console.RestAPIKeys.DefaultExpiration)

		id := testrand.UUID()
		now := time.Now()
		expires := time.Hour
		apiKey, _, err := service.CreateNoAuth(ctx, id, &expires)
		require.NoError(t, err)

		// test GetUserFromKey
		userID, exp, err := service.GetUserAndExpirationFromKey(ctx, apiKey)
		require.NoError(t, err)
		require.Equal(t, id, userID)
		require.False(t, exp.IsZero())
		require.False(t, exp.Before(now))

		// make sure an error is returned from duplicate apikey
		hash, err := service.HashKey(ctx, apiKey)
		require.NoError(t, err)
		_, err = service.InsertIntoDB(ctx, oidc.OAuthToken{
			UserID: id,
			Kind:   oidc.KindRESTTokenV0,
			Token:  hash,
		}, now, expires)
		require.True(t, restkeys.ErrDuplicateKey.Has(err))

		// test revocation
		require.NoError(t, service.Revoke(ctx, apiKey))
		token, err := sat.DB.OIDC().OAuthTokens().Get(ctx, oidc.KindRESTTokenV0, hash)
		require.Equal(t, sql.ErrNoRows, err)
		require.True(t, token.ExpiresAt.IsZero())

		// test revoke non existent key
		nonexistent := testrand.UUID().String()
		err = service.Revoke(ctx, nonexistent)
		require.Error(t, err)

		// test GetUserFromKey non existent key
		_, _, err = service.GetUserAndExpirationFromKey(ctx, nonexistent)
		require.True(t, restkeys.ErrInvalidKey.Has(err))
	})
}

func TestRESTKeysExpiration(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := restkeys.NewService(sat.DB.OIDC().OAuthTokens(), sat.Config.Console.RestAPIKeys.DefaultExpiration)
		now := time.Now()

		// test no expiration uses default
		expiresAt, err := service.InsertIntoDB(ctx, oidc.OAuthToken{
			UserID: testrand.UUID(),
			Kind:   oidc.KindRESTTokenV0,
			Token:  "testhash0",
		}, now, 0)
		require.NoError(t, err)
		require.Equal(t, now.Add(sat.Config.Console.RestAPIKeys.DefaultExpiration), expiresAt)

		// test negative expiration uses default
		expiresAt, err = service.InsertIntoDB(ctx, oidc.OAuthToken{
			UserID: testrand.UUID(),
			Kind:   oidc.KindRESTTokenV0,
			Token:  "testhash1",
		}, now, -10000)
		require.NoError(t, err)
		require.Equal(t, now.Add(sat.Config.Console.RestAPIKeys.DefaultExpiration), expiresAt)

		// test regular expiration
		expiration := 14 * time.Hour
		expiresAt, err = service.InsertIntoDB(ctx, oidc.OAuthToken{
			UserID: testrand.UUID(),
			Kind:   oidc.KindRESTTokenV0,
			Token:  "testhash2",
		}, now, expiration)
		require.NoError(t, err)
		require.Equal(t, now.Add(expiration), expiresAt)
	})
}
