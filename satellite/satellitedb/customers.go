// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that customers implements stripecoinpayments.CustomersDB.
var _ stripe.CustomersDB = (*customers)(nil)

// customers is an implementation of stripecoinpayments.CustomersDB.
//
// architecture: Database
type customers struct {
	db *satelliteDB
}

// Raw returns the raw dbx handle.
func (customers *customers) Raw() *dbx.DB {
	return customers.db.DB
}

// Insert inserts a stripe customer into the database.
func (customers *customers) Insert(ctx context.Context, userID uuid.UUID, customerID string) (err error) {
	defer mon.Task()(&ctx, userID, customerID)(&err)

	_, err = customers.db.Create_StripeCustomer(
		ctx,
		dbx.StripeCustomer_UserId(userID[:]),
		dbx.StripeCustomer_CustomerId(customerID),
		dbx.StripeCustomer_Create_Fields{},
	)

	return err
}

// GetCustomerID returns stripe customers id.
func (customers *customers) GetCustomerID(ctx context.Context, userID uuid.UUID) (_ string, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	idRow, err := customers.db.Get_StripeCustomer_CustomerId_By_UserId(ctx, dbx.StripeCustomer_UserId(userID[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", stripe.ErrNoCustomer
		}

		return "", err
	}

	return idRow.CustomerId, nil
}

// GetUserID return userID given stripe customer id.
func (customers *customers) GetUserID(ctx context.Context, customerID string) (_ uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)

	idRow, err := customers.db.Get_StripeCustomer_UserId_By_CustomerId(ctx, dbx.StripeCustomer_CustomerId(customerID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, stripe.ErrNoCustomer
		}

		return uuid.UUID{}, err
	}

	return uuid.FromBytes(idRow.UserId)
}

// List returns paginated customers id list, with customers created before specified date.
func (customers *customers) List(ctx context.Context, offset int64, limit int, before time.Time) (_ stripe.CustomersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page stripe.CustomersPage

	dbxCustomers, err := customers.db.Limited_StripeCustomer_By_CreatedAt_LessOrEqual_OrderBy_Desc_CreatedAt(ctx,
		dbx.StripeCustomer_CreatedAt(before),
		limit+1,
		offset,
	)
	if err != nil {
		return stripe.CustomersPage{}, err
	}

	if len(dbxCustomers) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)

		dbxCustomers = dbxCustomers[:len(dbxCustomers)-1]
	}

	for _, dbxCustomer := range dbxCustomers {
		cus, err := fromDBXCustomer(dbxCustomer)
		if err != nil {
			return stripe.CustomersPage{}, err
		}

		page.Customers = append(page.Customers, *cus)
	}

	return page, nil
}

// UpdatePackage updates the customer's package plan and purchase time.
func (customers *customers) UpdatePackage(ctx context.Context, userID uuid.UUID, packagePlan *string, timestamp *time.Time) (c *stripe.Customer, err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields := dbx.StripeCustomer_Update_Fields{
		PackagePlan:        dbx.StripeCustomer_PackagePlan_Null(),
		PurchasedPackageAt: dbx.StripeCustomer_PurchasedPackageAt_Null(),
	}
	if packagePlan != nil {
		updateFields.PackagePlan = dbx.StripeCustomer_PackagePlan(*packagePlan)
	}
	if timestamp != nil {
		updateFields.PurchasedPackageAt = dbx.StripeCustomer_PurchasedPackageAt(*timestamp)
	}

	dbxCustomer, err := customers.db.Update_StripeCustomer_By_UserId(ctx,
		dbx.StripeCustomer_UserId(userID[:]),
		updateFields,
	)
	if err != nil {
		return c, err
	}
	return fromDBXCustomer(dbxCustomer)
}

// UpdatePackage updates the customer's package plan and purchase time.
func (customers *customers) GetPackageInfo(ctx context.Context, userID uuid.UUID) (_ *string, _ *time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := customers.db.Get_StripeCustomer_PackagePlan_StripeCustomer_PurchasedPackageAt_By_UserId(ctx, dbx.StripeCustomer_UserId(userID[:]))
	if err != nil {
		return nil, nil, err
	}
	return row.PackagePlan, row.PurchasedPackageAt, nil
}

// fromDBXCustomer converts *dbx.StripeCustomer to *stripecoinpayments.Customer.
func fromDBXCustomer(dbxCustomer *dbx.StripeCustomer) (*stripe.Customer, error) {
	userID, err := uuid.FromBytes(dbxCustomer.UserId)
	if err != nil {
		return nil, err
	}

	return &stripe.Customer{
		ID:                 dbxCustomer.CustomerId,
		UserID:             userID,
		PackagePlan:        dbxCustomer.PackagePlan,
		PackagePurchasedAt: dbxCustomer.PurchasedPackageAt,
	}, nil
}
