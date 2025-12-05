// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestOAuthCodes(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		clientID, err := uuid.New()
		require.NoError(t, err)

		userID, err := uuid.New()
		require.NoError(t, err)

		// repositories
		codes := db.OIDC().OAuthCodes()

		start := time.Now()

		allCodes := []oidc.OAuthCode{
			{
				ClientID:  clientID,
				UserID:    userID,
				Code:      "expired",
				CreatedAt: start.Add(-2 * time.Hour),
				ExpiresAt: start.Add(-1 * time.Hour),
			},
			{
				ClientID:  clientID,
				UserID:    userID,
				Code:      "valid",
				CreatedAt: start,
				ExpiresAt: start.Add(time.Hour),
			},
			{
				ClientID:  clientID,
				UserID:    userID,
				Code:      "claimed",
				CreatedAt: start,
				ExpiresAt: start.Add(time.Hour),
			},
		}

		for _, code := range allCodes {
			err = codes.Create(ctx, code)
			require.NoError(t, err)
		}

		// claim this code ahead of time to test the token already claimed code path later on
		err = codes.Claim(ctx, "claimed")
		require.NoError(t, err)

		testCases := []struct {
			code string
			err  error
		}{
			{"expired", sql.ErrNoRows},
			{"valid", nil},
			{"claimed", sql.ErrNoRows}, // this should return an error since it was claimed above
		}

		for _, testCase := range testCases {
			_, err := codes.Get(ctx, testCase.code)
			require.Equal(t, testCase.err, err)
		}
	})
}

func TestOAuthTokens(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		clientID, err := uuid.New()
		require.NoError(t, err)

		userID, err := uuid.New()
		require.NoError(t, err)

		// repositories
		tokens := db.OIDC().OAuthTokens()

		start := time.Now()

		allTokens := []oidc.OAuthToken{
			{
				ClientID:  clientID,
				UserID:    userID,
				Kind:      oidc.KindAccessToken,
				Token:     "expired",
				CreatedAt: start.Add(-2 * time.Hour),
				ExpiresAt: start.Add(-1 * time.Hour),
			},
			{
				ClientID:  clientID,
				UserID:    userID,
				Kind:      oidc.KindRefreshToken,
				Token:     "valid",
				CreatedAt: start,
				ExpiresAt: start.Add(time.Hour),
			},
			{
				ClientID:  clientID,
				UserID:    userID,
				Kind:      oidc.KindRESTTokenV0,
				Token:     "testToken",
				CreatedAt: start,
				ExpiresAt: start.Add(time.Hour),
			},
		}

		for _, token := range allTokens {
			err = tokens.Create(ctx, token)
			require.NoError(t, err)
		}

		// ensure that creating an existing token doesn't cause an error
		err = tokens.Create(ctx, allTokens[1])
		require.NoError(t, err)

		testCases := []struct {
			kind  oidc.OAuthTokenKind
			token string
			err   error
		}{
			{oidc.KindAccessToken, "expired", sql.ErrNoRows},
			{oidc.KindRefreshToken, "valid", nil},
			{oidc.KindRESTTokenV0, "testToken", nil},
		}

		for _, testCase := range testCases {
			_, err := tokens.Get(ctx, testCase.kind, testCase.token)
			require.Equal(t, testCase.err, err)
			if testCase.kind == oidc.KindRESTTokenV0 {
				err = tokens.RevokeRESTTokenV0(ctx, testCase.token)
				require.NoError(t, err)
				token, err := tokens.Get(ctx, testCase.kind, testCase.token)
				require.Equal(t, sql.ErrNoRows, err)
				require.True(t, token.ExpiresAt.IsZero())
			}
		}
	})
}
