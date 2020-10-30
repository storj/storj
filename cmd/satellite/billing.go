// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
)

func runBillingCmd(ctx context.Context, cmdFunc func(context.Context, *stripecoinpayments.Service, *dbx.DB) error) error {
	// Open SatelliteDB for the Payment Service
	logger := zap.L()
	db, err := satellitedb.Open(ctx, logger.Named("db"), runCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// Open direct DB connection to execute custom queries
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
	defer func() {
		err = errs.Combine(err, dbxDB.Close())
	}()
	logger.Debug("Connected to:", zap.String("db source", source))

	payments, err := setupPayments(logger, db)
	if err != nil {
		return err
	}

	return cmdFunc(ctx, payments, dbxDB)
}

func setupPayments(log *zap.Logger, db satellite.DB) (*stripecoinpayments.Service, error) {
	pc := runCfg.Payments

	var stripeClient stripecoinpayments.StripeClient
	switch pc.Provider {
	default:
		stripeClient = stripecoinpayments.NewStripeMock(
			storj.NodeID{},
			db.StripeCoinPayments().Customers(),
			db.Console().Users(),
		)
	case "stripecoinpayments":
		stripeClient = stripecoinpayments.NewStripeClient(log, pc.StripeCoinPayments)
	}

	return stripecoinpayments.NewService(
		log.Named("payments.stripe:service"),
		stripeClient,
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
		pc.MinCoinPayment,
		pc.PaywallProportion)
}

// parseBillingPeriodFromString parses provided date string and returns corresponding time.Time.
func parseBillingPeriod(s string) (time.Time, error) {
	values := strings.Split(s, "/")

	if len(values) != 2 {
		return time.Time{}, errs.New("invalid date format %s, use mm/yyyy", s)
	}

	month, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse month: %v", err)
	}
	year, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil {
		return time.Time{}, errs.New("can not parse year: %v", err)
	}

	date := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if date.Year() != int(year) || date.Month() != time.Month(month) || date.Day() != 1 {
		return date, errs.New("dates mismatch have %s result %s", s, date)
	}

	return date, nil
}

// userData contains the uuid and email of a satellite user.
type userData struct {
	ID    uuid.UUID
	Email string
}

// generateStripeCustomers creates missing stripe-customers for users in our database.
func generateStripeCustomers(ctx context.Context) (err error) {
	return runBillingCmd(ctx, func(ctx context.Context, payments *stripecoinpayments.Service, dbxDB *dbx.DB) error {
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
			var user userData
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
