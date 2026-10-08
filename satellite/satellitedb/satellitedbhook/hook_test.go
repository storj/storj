// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedbhook_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbhook"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/mud"
)

func TestMethod_Validation(t *testing.T) {
	ctx := context.Background()
	noop := func(context.Context, uuid.UUID) error { return nil }
	noopAfter := func(_ context.Context, _ uuid.UUID, _ *console.User, err error) error { return err }

	require.PanicsWithValue(t, `satellitedbhook: console.User is not an interface`, func() {
		satellitedbhook.Before[console.User](ctx, "Get", noop)
	})
	require.PanicsWithValue(t, `satellitedbhook: console.Users does not have method "Missing"`, func() {
		satellitedbhook.Before[console.Users](ctx, "Missing", noop)
	})
	require.PanicsWithValue(t, `satellitedbhook: console.DB.Users does not take context.Context as first argument`, func() {
		satellitedbhook.Before[console.DB](ctx, "Users", noop)
	})

	require.PanicsWithValue(t, `satellitedbhook: hook for console.Users.Get must be func(context.Context, uuid.UUID) error, got func(context.Context) error`, func() {
		satellitedbhook.Before[console.Users](ctx, "Get", func(context.Context) error { return nil })
	})
	require.PanicsWithValue(t, `satellitedbhook: hook for console.Users.Get must be func(context.Context, uuid.UUID, *console.User, error) error, got func(context.Context, uuid.UUID) error`, func() {
		satellitedbhook.After[console.Users](ctx, "Get", noop)
	})

	_, done := satellitedbhook.Before[console.Users](ctx, "Get", noop)
	require.PanicsWithValue(t, "unused method hook: console.Users.Get", done)

	_, done = satellitedbhook.After[console.Users](ctx, "Get", noopAfter)
	require.PanicsWithValue(t, "unused method hook: console.Users.Get", done)
}

func TestMethod(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		consoleDB := satellitedbhook.Wrap(db).Console()

		user, err := consoleDB.Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			FullName:     "User",
			Email:        "hook@mail.test",
			PasswordHash: []byte("password"),
			ProjectLimit: 3,
		})
		require.NoError(t, err)

		t.Run("error", func(t *testing.T) {
			hookErr := errors.New("hook")
			var gotID uuid.UUID
			hookCtx, done := satellitedbhook.Before[console.Users](ctx, "Get",
				func(_ context.Context, id uuid.UUID) error { gotID = id; return hookErr })
			defer done()

			_, err := consoleDB.Users().Get(hookCtx, user.ID)
			require.ErrorIs(t, err, hookErr)
			require.Equal(t, user.ID, gotID)

			// calls without the hook context are not affected.
			_, err = consoleDB.Users().Get(ctx, user.ID)
			require.NoError(t, err)
		})

		t.Run("after", func(t *testing.T) {
			hookErr := errors.New("hook")
			var order []string
			hookCtx, doneBefore := satellitedbhook.Before[console.Users](ctx, "Get",
				func(context.Context, uuid.UUID) error { order = append(order, "before"); return nil })
			defer doneBefore()
			hookCtx, doneAfter := satellitedbhook.After[console.Users](hookCtx, "Get",
				func(_ context.Context, id uuid.UUID, got *console.User, err error) error {
					order = append(order, "after")
					require.NoError(t, err)
					require.Equal(t, user.ID, id)
					require.Equal(t, user.Email, got.Email)
					return hookErr
				})
			defer doneAfter()

			_, err := consoleDB.Users().Get(hookCtx, user.ID)
			require.ErrorIs(t, err, hookErr)
			require.Equal(t, []string{"before", "after"}, order)
		})

		t.Run("after receives error", func(t *testing.T) {
			var got error
			hookCtx, done := satellitedbhook.After[console.Users](ctx, "Get",
				func(_ context.Context, _ uuid.UUID, _ *console.User, err error) error { got = err; return nil })
			defer done()

			_, err := consoleDB.Users().Get(hookCtx, testrand.UUID())
			require.NoError(t, err)
			require.Error(t, got)
		})

		t.Run("embedded interface", func(t *testing.T) {
			// console.DBTx embeds console.DB, so hooks on console.DB apply to transactions as well.
			var calls int
			hookCtx, done := satellitedbhook.Before[console.DB](ctx, "WithTx",
				func(context.Context, func(context.Context, console.DBTx) error) error { calls++; return nil })
			defer done()

			err := consoleDB.WithTx(hookCtx, func(ctx context.Context, tx console.DBTx) error {
				return tx.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error { return nil })
			})
			require.NoError(t, err)
			require.Equal(t, 2, calls)
		})

		t.Run("transaction", func(t *testing.T) {
			var called bool
			hookCtx, done := satellitedbhook.Before[console.Users](ctx, "Get",
				func(context.Context, uuid.UUID) error { called = true; return nil })
			defer done()

			err := consoleDB.WithTx(hookCtx, func(ctx context.Context, tx console.DBTx) error {
				_, err := tx.Users().Get(ctx, user.ID)
				return err
			})
			require.NoError(t, err)
			require.True(t, called)
		})
	})
}

func TestModule(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		ball := mud.NewBall()
		mud.Supply[satellite.DB](ball, db)
		satellitedbhook.Module(ball)
		mud.View[satellite.DB, console.DB](ball, satellite.DB.Console)
		require.NoError(t, mud.ForEach(ball, mud.Initialize(ctx), mud.All))

		var called bool
		hookCtx, done := satellitedbhook.Before[console.Users](ctx, "Get",
			func(context.Context, uuid.UUID) error { called = true; return nil })
		defer done()

		_, _ = mud.MustLookup[console.DB](ball).Users().Get(hookCtx, testrand.UUID())
		require.True(t, called)
	})
}
