// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// setup creates necessary services and connections for billing sub commands.
func setup(dbConnection string, pc paymentsconfig.Config) (*stripecoinpayments.Service, *dbx.DB, error) {
	// Open SatelliteDB for the Payment Service
	logger := zap.L()

	db, err := satellitedb.New(logger.Named("db"), dbConnection, satellitedb.Options{})
	if err != nil {
		return nil, nil, errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// Open direct DB connection to execute custom queries
	driver, source, implementation, err := dbutil.SplitConnStr(dbConnection)
	if err != nil {
		return nil, nil, err
	}
	if implementation != dbutil.Postgres && implementation != dbutil.Cockroach {
		return nil, nil, errs.New("unsupported driver %q", driver)
	}

	dbxDB, err := dbx.Open(driver, source)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		err = errs.Combine(err, dbxDB.Close())
	}()
	logger.Debug("Connected to:", zap.String("db source", source))

	stripeClient := stripecoinpayments.NewStripeClient(logger, pc.StripeCoinPayments)

	service, err := stripecoinpayments.NewService(
		logger.Named("payments.stripe:service"),
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
		pc.MinCoinPayment)
	if err != nil {
		return nil, nil, err
	}

	return service, dbxDB, nil
}

// PrepareCustomerInvoiceItems creates usage reports for customer projects for the specified month in the database.
func PrepareCustomerInvoiceItems(ctx context.Context, period, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	periodT, err := parseBillingPeriod(period)
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return payments.InvoiceApplyProjectRecords(ctx, periodT)
}

// CreateCustomerInvoiceItems creates customer invoice items for the specified month on Stripe.
func CreateCustomerInvoiceItems(ctx context.Context, period, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	periodT, err := parseBillingPeriod(period)
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return payments.InvoiceApplyProjectRecords(ctx, periodT)
}

// CreateCustomerInvoiceCoupons applies customer coupons for the specified month on Stripe.
func CreateCustomerInvoiceCoupons(ctx context.Context, period, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	periodT, err := parseBillingPeriod(period)
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return payments.InvoiceApplyCoupons(ctx, periodT)
}

// CreateCustomerInvoiceCredits applies customer credits for the specified month on Stripe.
func CreateCustomerInvoiceCredits(ctx context.Context, period, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	periodT, err := parseBillingPeriod(period)
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return payments.InvoiceApplyCredits(ctx, periodT)
}

// CreateCustomerInvoices creates customer invoices for the specified month on Stripe.
func CreateCustomerInvoices(ctx context.Context, period, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	periodT, err := parseBillingPeriod(period)
	if err != nil {
		return errs.New("invalid period specified: %v", err)
	}

	return payments.CreateInvoices(ctx, periodT)
}

// FinalizeCustomerInvoices sets the auto-advance flag on all draft invoices currently available on Stripe.
func FinalizeCustomerInvoices(ctx context.Context, dbConnection string, pc paymentsconfig.Config) error {
	payments, _, err := setup(dbConnection, pc)
	if err != nil {
		return err
	}

	return payments.FinalizeInvoices(ctx)
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
