// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that customers implements stripecoinpayments.CustomersDB.
var _ stripecoinpayments.CustomersDB = (*customers)(nil)

// customers is an implementation of stripecoinpayments.CustomersDB.
//
// architecture: Database
type customers struct {
	db *satelliteDB
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
		if err == sql.ErrNoRows {
			return "", stripecoinpayments.ErrNoCustomer
		}

		return "", err
	}

	return idRow.CustomerId, nil
}

// List returns paginated customers id list, with customers created before specified date.
func (customers *customers) List(ctx context.Context, offset int64, limit int, before time.Time) (_ stripecoinpayments.CustomersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page stripecoinpayments.CustomersPage

	dbxCustomers, err := customers.db.Limited_StripeCustomer_By_CreatedAt_LessOrEqual_OrderBy_Desc_CreatedAt(ctx,
		dbx.StripeCustomer_CreatedAt(before),
		limit+1,
		offset,
	)
	if err != nil {
		return stripecoinpayments.CustomersPage{}, err
	}

	if len(dbxCustomers) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit) + 1

		dbxCustomers = dbxCustomers[:len(dbxCustomers)-1]
	}

	for _, dbxCustomer := range dbxCustomers {
		cus, err := fromDBXCustomer(dbxCustomer)
		if err != nil {
			return stripecoinpayments.CustomersPage{}, err
		}

		page.Customers = append(page.Customers, *cus)
	}

	return page, nil
}

// fromDBXCustomer converts *dbx.StripeCustomer to *stripecoinpayments.Customer.
func fromDBXCustomer(dbxCustomer *dbx.StripeCustomer) (*stripecoinpayments.Customer, error) {
	userID, err := dbutil.BytesToUUID(dbxCustomer.UserId)
	if err != nil {
		return nil, err
	}

	return &stripecoinpayments.Customer{
		ID:     dbxCustomer.CustomerId,
		UserID: userID,
	}, nil
}
