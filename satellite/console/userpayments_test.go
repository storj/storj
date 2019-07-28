// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUserPaymentInfos(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()

		customerID := testrand.Bytes(8)
		passHash := testrand.Bytes(8)

		// create user
		user, err := consoleDB.Users().Insert(ctx, &console.User{
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: passHash,
			Status:       console.Active,
		})
		require.NoError(t, err)

		t.Run("create user payment info", func(t *testing.T) {
			info, err := consoleDB.UserPayments().Create(ctx, console.UserPayment{
				UserID:     user.ID,
				CustomerID: customerID,
			})

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID, info.CustomerID)
		})

		t.Run("get user payment info", func(t *testing.T) {
			info, err := consoleDB.UserPayments().Get(ctx, user.ID)

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID, info.CustomerID)
		})
	})
}
