// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

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

// GetStripeIDs returns stripe customer and billing ids.
func (customers *customers) GetStripeIDs(ctx context.Context, userID uuid.UUID) (billingID *string, customerID string, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	idRow, err := customers.db.Get_StripeCustomer_CustomerId_StripeCustomer_BillingCustomerId_By_UserId(ctx, dbx.StripeCustomer_UserId(userID[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", stripe.ErrNoCustomer
		}

		return nil, "", err
	}

	return idRow.BillingCustomerId, idRow.CustomerId, nil
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
func (customers *customers) List(ctx context.Context, userIDCursor uuid.UUID, limit int, before time.Time) (page stripe.CustomersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := customers.db.QueryContext(ctx, customers.db.Rebind(`
		SELECT
			stripe_customers.user_id, stripe_customers.customer_id, stripe_customers.billing_customer_id, stripe_customers.package_plan, stripe_customers.purchased_package_at
		FROM
			stripe_customers
		WHERE
			stripe_customers.user_id > ? AND
			stripe_customers.created_at < ?
		ORDER BY stripe_customers.user_id ASC
		LIMIT ?
	`), userIDCursor, before, limit+1)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return stripe.CustomersPage{}, nil
		}
		return stripe.CustomersPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	results := []stripe.Customer{}
	for rows.Next() {
		var customer stripe.Customer
		err := rows.Scan(&customer.UserID, &customer.ID, &customer.BillingID, &customer.PackagePlan, &customer.PackagePurchasedAt)
		if err != nil {
			return stripe.CustomersPage{}, errs.New("unable to get stripe customer: %+v", err)
		}

		results = append(results, customer)
	}
	if err := rows.Err(); err != nil {
		return stripe.CustomersPage{}, errs.New("error while listing stripe customers: %+v", err)
	}

	if len(results) == limit+1 {
		results = results[:len(results)-1]

		page.Next = true
		page.Cursor = results[len(results)-1].UserID
	}

	page.Customers = results
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
		BillingID:          dbxCustomer.BillingCustomerId,
		PackagePlan:        dbxCustomer.PackagePlan,
		PackagePurchasedAt: dbxCustomer.PurchasedPackageAt,
	}, nil
}
