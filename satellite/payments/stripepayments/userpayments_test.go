// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripepayments_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/stripepayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUserPaymentInfos(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()
		stripeDB := db.StripePayments()

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
			info, err := stripeDB.UserPayments().Create(ctx, stripepayments.UserPayment{
				UserID:     user.ID,
				CustomerID: customerID,
			})

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID, info.CustomerID)
		})

		t.Run("get user payment info", func(t *testing.T) {
			info, err := stripeDB.UserPayments().Get(ctx, user.ID)

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID, info.CustomerID)
		})
	})
}
