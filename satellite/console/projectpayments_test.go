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

func TestProjectPaymentInfos(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()

		var customerID [8]byte
		_, err := rand.Read(customerID[:])
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create customer id: %s", err))
		}

		var paymentMethodID [8]byte
		_, err = rand.Read(paymentMethodID[:])
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create payment method id: %s", err))
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

		// create user payment info
		userPmInfo, err := consoleDB.UserPayments().Create(ctx, console.UserPayment{
			UserID:     user.ID,
			CustomerID: customerID[:],
		})
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create user payment info: %s", err))
		}

		// create project
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			Name: "test",
		})
		if err != nil {
			t.Fatal(fmt.Sprintf("can not create project: %s", err))
		}

		t.Run("create project payment info", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().Create(ctx, console.ProjectPayment{
				ProjectID:       proj.ID,
				PayerID:         userPmInfo.UserID,
				PaymentMethodID: paymentMethodID[:],
			})

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID[:], info.PaymentMethodID)
		})

		t.Run("get by project id", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().GetByProjectID(ctx, proj.ID)

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID[:], info.PaymentMethodID)
		})

		t.Run("get by payer id", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().GetByPayerID(ctx, userPmInfo.UserID)

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID[:], info.PaymentMethodID)
		})
	})
}
