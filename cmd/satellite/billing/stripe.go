// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

// UserData contains the uuid and email of a satellite user.
type UserData struct {
	ID    uuid.UUID
	Email string
}

// GenerateStripeCustomers creates missing stripe-customers for users in our database.
func GenerateStripeCustomers(ctx context.Context, dbConnection string, pc paymentsconfig.Config) (err error) {
	payments, db, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	rows, err := db.Query(ctx, "SELECT id, email FROM users WHERE id NOT IN (SELECT user_id from stripe_customers) AND users.status=1")
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

		err = payments.Accounts().Setup(ctx, user.ID, user.Email)
		if err != nil {
			return err
		}

	}

	zap.L().Info("Ensured Stripe-Customer", zap.Int64("created", n))

	return err
}
