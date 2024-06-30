// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb"
)

func runBillingCmd(ctx context.Context, cmdFunc func(context.Context, *stripe.Service, satellite.DB) error) error {
	// Open SatelliteDB for the Payment Service
	logger := zap.L()
	db, err := satellitedb.Open(ctx, logger.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-billing"})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	payments, err := setupPayments(logger, db)
	if err != nil {
		return err
	}

	return cmdFunc(ctx, payments, db)
}

func setupPayments(log *zap.Logger, db satellite.DB) (*stripe.Service, error) {
	pc := runCfg.Payments

	var stripeClient stripe.Client
	switch pc.Provider {
	case "": // just new mock, only used in testing binaries
		stripeClient = stripe.NewStripeMock(
			db.StripeCoinPayments().Customers(),
			db.Console().Users(),
		)
	case "stripecoinpayments":
		stripeClient = stripe.NewStripeClient(log, pc.StripeCoinPayments)
	default:
		return nil, errs.New("invalid stripe coin payments provider %q", pc.Provider)
	}

	prices, err := pc.UsagePrice.ToModel()
	if err != nil {
		return nil, err
	}

	priceOverrides, err := pc.UsagePriceOverrides.ToModels()
	if err != nil {
		return nil, err
	}

	return stripe.NewService(
		log.Named("payments.stripe:service"),
		stripeClient,
		pc.StripeCoinPayments,
		db.StripeCoinPayments(),
		db.Wallets(),
		db.Billing(),
		db.Console().Projects(),
		db.Console().Users(),
		db.ProjectAccounting(),
		prices,
		priceOverrides,
		pc.PackagePlans.Packages,
		pc.BonusRate,
		analytics.NewService(log.Named("analytics:service"), runCfg.Analytics, runCfg.Console.SatelliteName),
		emission.NewService(runCfg.Emission),
		runCfg.Console.SelfServeAccountDeleteEnabled,
	)
}

// parseYearMonth parses year and month from the provided string and returns a corresponding time.Time for the first day
// of the month. The input year and month should be iso8601 format (yyyy-mm).
func parseYearMonth(yearMonth string) (time.Time, error) {
	// parse using iso8601 yyyy-mm
	t, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return time.Time{}, errs.New("invalid date specified. accepted format is yyyy-mm: %v", err)
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// generateStripeCustomers creates missing stripe-customers for users in our database.
func generateStripeCustomers(ctx context.Context) (err error) {
	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, db satellite.DB) error {
		accounts := payments.Accounts()

		users, err := db.StripeCoinPayments().Customers().ListMissingCustomers(ctx)
		if err != nil {
			return err
		}

		for _, user := range users {
			_, err = accounts.Setup(ctx, user.ID, user.Email, user.SignupPromoCode)
			if err != nil {
				return err
			}
		}

		zap.L().Info("Ensured Stripe-Customer", zap.Int("created", len(users)))

		return err
	})
}

func cmdApplyFreeTierCoupons(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	return runBillingCmd(ctx, func(ctx context.Context, payments *stripe.Service, _ satellite.DB) error {
		return payments.ApplyFreeTierCoupons(ctx)
	})
}
