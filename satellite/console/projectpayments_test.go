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

func TestProjectPaymentInfos(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		consoleDB := db.Console()

		customerID := testrand.Bytes(8)
		paymentMethodID := testrand.Bytes(8)
		passHash := testrand.Bytes(8)

		// create user
		user, err := consoleDB.Users().Insert(ctx, &console.User{
			FullName:     "John Doe",
			Email:        "john@mail.test",
			PasswordHash: passHash,
			Status:       console.Active,
		})
		require.NoError(t, err)

		// create user payment info
		userPmInfo, err := consoleDB.UserPayments().Create(ctx, console.UserPayment{
			UserID:     user.ID,
			CustomerID: customerID,
		})
		require.NoError(t, err)

		// create project
		proj, err := consoleDB.Projects().Insert(ctx, &console.Project{
			Name: "test",
		})
		require.NoError(t, err)

		t.Run("create project payment info", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().Create(ctx, console.ProjectPayment{
				ProjectID:       proj.ID,
				PayerID:         userPmInfo.UserID,
				PaymentMethodID: paymentMethodID,
			})

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID, info.PaymentMethodID)
		})

		t.Run("get by project id", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().GetByProjectID(ctx, proj.ID)

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID, info.PaymentMethodID)
		})

		t.Run("get by payer id", func(t *testing.T) {
			info, err := consoleDB.ProjectPayments().GetByPayerID(ctx, userPmInfo.UserID)

			assert.NoError(t, err)
			assert.Equal(t, proj.ID, info.ProjectID)
			assert.Equal(t, userPmInfo.UserID, info.PayerID)
			assert.Equal(t, paymentMethodID, info.PaymentMethodID)
		})
	})
}
