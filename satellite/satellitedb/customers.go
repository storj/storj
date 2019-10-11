// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// customers is an implementation of stripecoinpayments.CustomersDB.
type customers struct {
	db *dbx.DB
}

// Insert inserts a stripe customer into the database.
func (customers *customers) Insert(ctx context.Context, userID uuid.UUID, customerID string) (err error) {
	defer mon.Task()(&ctx, userID, customerID)(&err)

	_, err = customers.db.Create_StripeCustomers(
		ctx,
		dbx.StripeCustomers_UserId(userID[:]),
		dbx.StripeCustomers_CustomerId(customerID),
	)

	return err
}

// GetCustomerID return stripe customers id.
func (customers *customers) GetCustomerID(ctx context.Context, userID uuid.UUID) (customerID string, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	idRow, err := customers.db.Get_StripeCustomers_CustomerId_By_UserId(ctx, dbx.StripeCustomers_UserId(userID[:]))
	if err != nil {
		return "", err
	}

	return idRow.CustomerId, nil
}
