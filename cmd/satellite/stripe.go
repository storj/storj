// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// UserData contains the uuid and email of a satellite user.
type UserData struct {
	ID    uuid.UUID
	Email string
}

// generateStripeCustomers creates missing stripe-customers for users in our database.
func generateStripeCustomers(ctx context.Context) (err error) {
	return runBillingCmd(func(payments *stripecoinpayments.Service, dbxDB *dbx.DB) error {
		accounts := payments.Accounts()

		rows, err := dbxDB.Query(ctx, "SELECT id, email FROM users WHERE id NOT IN (SELECT user_id from stripe_customers) AND users.status=1")
		if err != nil {
			return err
		}
		defer func() {
			err = errs.Combine(err, rows.Close())
		}()

		var n int64
		for rows.Next() {
			n++
			var user UserData
			err := rows.Scan(&user.ID, &user.Email)
			if err != nil {
				return err
			}

			err = accounts.Setup(ctx, user.ID, user.Email)
			if err != nil {
				return err
			}

		}

		zap.L().Info("Ensured Stripe-Customer", zap.Int64("created", n))

		return err
	})
}
