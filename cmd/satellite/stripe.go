// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/prometheus/common/log"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// Payments is a wrapper around the Account Handling Service
type Payments struct {
	Accounts payments.Accounts
}

// UserData contains the uuid and email of a satellite user
type UserData struct {
	ID    uuid.UUID
	Email string
}

// generateStripeCustomers creates missing stripe-customers for users in our database
func generateStripeCustomers(ctx context.Context) error {
	//Open SatelliteDB for the Payment Service
	db, err := satellitedb.New(zap.L().Named("db"), runCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	//Open direct DB connection to execute custom queries
	driver, source, implementation, err := dbutil.SplitConnStr(runCfg.Database)
	if err != nil {
		return err
	}
	if implementation != dbutil.Postgres && implementation != dbutil.Cockroach {
		return errs.New("unsupported driver %q", driver)
	}

	dbxDB, err := dbx.Open(driver, source)
	if err != nil {
		return errs.New("failed opening database via DBX at %q: %v",
			source, err)
	}
	log.Debug("Connected to:", zap.String("db source", source))
	defer func() {
		err = errs.Combine(err, dbxDB.Close())
	}()

	handler, err := setupPayments(zap.L().Named("payments"), db)
	if err != nil {
		return err
	}

	rows, err := dbxDB.Query(ctx, "SELECT id, email FROM users WHERE email NOT IN (SELECT email from stripe_coinpayments")
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var user UserData
		err := rows.Scan(&user)
		if err != nil {
			return err
		}

		err = handler.Accounts.Setup(ctx, user.ID, user.Email)
		if err != nil {
			return err
		}
	}
	return err
}

func setupPayments(log *zap.Logger, db satellite.DB) (handler *Payments, err error) {
	pc := runCfg.Payments
	service, err := stripecoinpayments.NewService(
		log.Named("payments.stripe:service"),
		pc.StripeCoinPayments,
		db.StripeCoinPayments(),
		db.Console().Projects(),
		db.ProjectAccounting(),
		pc.StorageTBPrice,
		pc.EgressTBPrice,
		pc.ObjectPrice,
		pc.BonusRate,
		pc.CouponValue,
		pc.CouponDuration,
		pc.CouponProjectLimit,
		pc.MinCoinPayment)

	if err != nil {
		return nil, err
	}

	handler = &Payments{}
	handler.Accounts = service.Accounts()

	return handler, err
}
