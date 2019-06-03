// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestUserPaymentInfos(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()

		var customerID [8]byte
		_, err := rand.Read(customerID[:])
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create customer id: %s", err))
		}

		var passHash [8]byte
		_, err = rand.Read(passHash[:])
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create password hash for user: %s", err))
		}

		// create user
		user, err := consoleDB.Users().Insert(ctx, &console.User{
			FullName:     "John Doe",
			Email:        "john@example.com",
			PasswordHash: passHash[:],
			Status:       console.Active,
		})
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create user: %s", err))
		}

		t.Run("create user payment info", func(t *testing.T) {
			info, err := consoleDB.UserPaymentInfos().Create(ctx, console.UserPaymentInfo{
				UserID:     user.ID,
				CustomerID: customerID[:],
			})

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID[:], info.CustomerID)
		})

		t.Run("get user payment info", func(t *testing.T) {
			info, err := consoleDB.UserPaymentInfos().Get(ctx, user.ID)

			assert.NoError(t, err)
			assert.Equal(t, user.ID, info.UserID)
			assert.Equal(t, customerID[:], info.CustomerID)
		})
	})
}
