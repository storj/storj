// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// UserData contains the uuid and email of a satellite user.
type UserData struct {
	ID    []byte
	Email string
}

// generateStripeCustomers creates missing stripe-customers for users in our database.
func generateStripeCustomers(ctx context.Context) (err error) {
	//Open SatelliteDB for the Payment Service
	logger := zap.L()
	db, err := satellitedb.New(logger.Named("db"), runCfg.Database, satellitedb.Options{})
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
		return err
	}
	logger.Debug("Connected to:", zap.String("db source", source))
	defer func() {
		err = errs.Combine(err, dbxDB.Close())
	}()

	handler, err := setupPayments(zap.L().Named("payments"), db)
	if err != nil {
		return err
	}

	rows, err := dbxDB.Query(ctx, "SELECT id, email FROM users WHERE id NOT IN (SELECT user_id from stripe_customers)")
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
		uid, err := dbutil.BytesToUUID(user.ID)
		if err != nil {
			return err
		}
		err = handler.Setup(ctx, uid, user.Email)
		if err != nil {
			return err
		}
	}
	logger.Info("Ensured Stripe-Customer", zap.Int64("created", n))
	return err
}

func setupPayments(log *zap.Logger, db satellite.DB) (handler payments.Accounts, err error) {
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

	handler = service.Accounts()

	return handler, err
}
