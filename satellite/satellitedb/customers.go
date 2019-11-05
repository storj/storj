// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that customers implements stripecoinpayments.CustomersDB.
var _ stripecoinpayments.CustomersDB = (*customers)(nil)

// customers is an implementation of stripecoinpayments.CustomersDB.
type customers struct {
	db *dbx.DB
}

// Insert inserts a stripe customer into the database.
func (customers *customers) Insert(ctx context.Context, userID uuid.UUID, customerID string) (err error) {
	defer mon.Task()(&ctx, userID, customerID)(&err)

	_, err = customers.db.Create_StripeCustomer(
		ctx,
		dbx.StripeCustomer_UserId(userID[:]),
		dbx.StripeCustomer_CustomerId(customerID),
	)

	return err
}

// GetCustomerID returns stripe customers id.
func (customers *customers) GetCustomerID(ctx context.Context, userID uuid.UUID) (_ string, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	idRow, err := customers.db.Get_StripeCustomer_CustomerId_By_UserId(ctx, dbx.StripeCustomer_UserId(userID[:]))
	if err != nil {
		return "", err
	}

	return idRow.CustomerId, nil
}
