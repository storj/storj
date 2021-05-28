// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
)

func TestService_GetOrCreate(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authorizationDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authorizationDB.Close)

	service := NewService(zaptest.NewLogger(t), authorizationDB)
	require.NotNil(t, service)

	{ // new user, no existing authorization tokens (create)
		userID := "new@mail.test"
		group, err := authorizationDB.Get(ctx, userID)
		require.Error(t, err, ErrNotFound)
		require.Empty(t, group)

		token, err := service.GetOrCreate(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, token)

		require.Equal(t, userID, token.UserID)
	}

	{ // existing user with unclaimed authorization token (get)
		userID := "old@mail.test"
		group, err := authorizationDB.Create(ctx, userID, 1)
		require.NoError(t, err)
		require.NotEmpty(t, group)
		require.Len(t, group, 1)

		existingAuth := group[0]

		token, err := service.GetOrCreate(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, token)

		require.Equal(t, userID, token.UserID)
		require.Equal(t, existingAuth.Token, *token)
	}
}

func TestService_GetOrCreate_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authorizationDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authorizationDB.Close)

	service := NewService(zaptest.NewLogger(t), authorizationDB)
	require.NotNil(t, service)

	{ // empty user ID
		token, err := service.GetOrCreate(ctx, "")
		require.Error(t, errs.Unwrap(err), ErrEmptyUserID.Error())
		require.Nil(t, token)
	}
}
