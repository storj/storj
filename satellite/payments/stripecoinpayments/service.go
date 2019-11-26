// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var (
	// Error defines stripecoinpayments service error.
	Error = errs.Class("stripecoinpayments service error")
	// ErrNoCouponUsages indicates that there are no coupon usages.
	ErrNoCouponUsages = errs.Class("stripecoinpayments no coupon usages")

	mon = monkit.Package()
)

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey              string        `help:"stripe API secret key" default:""`
	StripePublicKey              string        `help:"stripe API public key" default:""`
	CoinpaymentsPublicKey        string        `help:"coinpayments API public key" default:""`
	CoinpaymentsPrivateKey       string        `help:"coinpayments API private key key" default:""`
	TransactionUpdateInterval    time.Duration `help:"amount of time we wait before running next transaction update loop" devDefault:"1m" releaseDefault:"30m"`
	AccountBalanceUpdateInterval time.Duration `help:"amount of time we wait before running next account balance update loop" devDefault:"3m" releaseDefault:"1h30m"`
	ConversionRatesCycleInterval time.Duration `help:"amount of time we wait before running next conversion rates update loop" devDefault:"1m" releaseDefault:"10m"`
	// TODO: uncomment when coupons will be finished.
	// CouponUsageCycleInterval     time.Duration `help:"amount of time we wait before running next coupon usage update loop" devDefault:"1d" releaseDefault:"1d"`
}

// Service is an implementation for payment service via Stripe and Coinpayments.
//
// architecture: Service
type Service struct {
	log          *zap.Logger
	db           DB
	projectsDB   console.Projects
	usageDB      accounting.ProjectAccounting
	stripeClient *client.API
	coinPayments *coinpayments.Client

	PerObjectPrice int64
	EgressPrice    int64
	TBhPrice       int64

	mu       sync.Mutex
	rates    coinpayments.CurrencyRateInfos
	ratesErr error
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, config Config, db DB, projectsDB console.Projects, usageDB accounting.ProjectAccounting, perObjectPrice, egressPrice, tbhPrice int64) *Service {
	backendConfig := &stripe.BackendConfig{
		LeveledLogger: log.Sugar(),
	}

	stripeClient := client.New(config.StripeSecretKey,
		&stripe.Backends{
			API:     stripe.GetBackendWithConfig(stripe.APIBackend, backendConfig),
			Connect: stripe.GetBackendWithConfig(stripe.ConnectBackend, backendConfig),
			Uploads: stripe.GetBackendWithConfig(stripe.UploadsBackend, backendConfig),
		},
	)

	coinPaymentsClient := coinpayments.NewClient(
		coinpayments.Credentials{
			PublicKey:  config.CoinpaymentsPublicKey,
			PrivateKey: config.CoinpaymentsPrivateKey,
		},
	)

	return &Service{
		log:            log,
		db:             db,
		projectsDB:     projectsDB,
		usageDB:        usageDB,
		stripeClient:   stripeClient,
		coinPayments:   coinPaymentsClient,
		TBhPrice:       tbhPrice,
		PerObjectPrice: perObjectPrice,
		EgressPrice:    egressPrice,
	}
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

// AddCoupon attaches a coupon for payment account.
func (service *Service) AddCoupon(ctx context.Context, userID, projectID uuid.UUID, amount int64, duration time.Duration, description string) (err error) {
	defer mon.Task()(&ctx, userID, amount, duration)(&err)

	coupon := payments.Coupon{
		UserID:      userID,
		Status:      payments.CouponActive,
		ProjectID:   projectID,
		Amount:      amount,
		Description: description,
		Duration:    duration,
	}

	return Error.Wrap(service.db.Coupons().Insert(ctx, coupon))
}

// TODO: uncomment when coupons will be ready.
// updateCouponUsageLoop updates all daily coupon usage in a loop.
//func (service *Service) updateCouponUsageLoop(ctx context.Context) (err error) {
//	defer mon.Task()(&ctx)(&err)
//
//	const limit = 25
//	before := time.Now()
//
//	// takes first coupon page
//	couponPage, err := service.db.Coupons().ListPaged(ctx, 0, limit, before, payments.CouponActive)
//	if err != nil {
//		return Error.Wrap(err)
//	}
//
//	// iterates through all coupons, takes daily project usage and create new coupon usage
//	err = service.createDailyCouponUsage(ctx, couponPage.Coupons)
//	if err != nil {
//		return Error.Wrap(err)
//	}
//
//	// iterates by rest pages
//	for couponPage.Next {
//		if err = ctx.Err(); err != nil {
//			return Error.Wrap(err)
//		}
//
//		couponPage, err = service.db.Coupons().ListPaged(ctx, couponPage.NextOffset, limit, before, payments.CouponActive)
//		if err != nil {
//			return Error.Wrap(err)
//		}
//
//		// iterates through all coupons, takes daily project usage and create new coupon usage
//		err = service.createDailyCouponUsage(ctx, couponPage.Coupons)
//		if err != nil {
//			return Error.Wrap(err)
//		}
//	}
//
//	return nil
//}

// createDailyCouponUsage iterates through all coupons, takes daily project usage and create new coupon usage.
// TODO: it will works only for 1 coupon per project. Need rework in future.
//func (service *Service) createDailyCouponUsage(ctx context.Context, coupons []payments.Coupon) (err error) {
//	defer mon.Task()(&ctx)(&err)
//
//	for _, coupon := range coupons {
//		// check if coupon expired
//		if coupon.Created.Add(coupon.Duration).After(time.Now().UTC()) {
//			if err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponExpired); err != nil {
//				return err
//			}
//
//			continue
//		}
//
//		since, err := service.db.Coupons().GetLatest(ctx, coupon.ID)
//		if err != nil {
//			if !ErrNoCouponUsages.Has(err) {
//				return err
//			}
//
//			since = coupon.Created
//		}
//
//		start, end := date.DayBoundary(since)
//		usage, err := service.usageDB.GetProjectTotal(ctx, coupon.ProjectID, start, end)
//		if err != nil {
//			return err
//		}
//
//		// TODO: reuse this code fragment.
//		egressPrice := usage.Egress * service.EgressPrice / int64(memory.TB)
//		objectCountPrice := int64(usage.ObjectCount * float64(service.PerObjectPrice))
//		storageGbHrsPrice := int64(usage.Storage*float64(service.TBhPrice)) / int64(memory.TB)
//
//		currentUsageAmount := egressPrice + objectCountPrice + storageGbHrsPrice
//
//		// TODO: we should add caching for TotalUsage call
//		alreadyChargedAmount, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
//		if err != nil {
//			return err
//		}
//		remaining := coupon.Amount - alreadyChargedAmount
//
//		// check if coupon is used
//		if currentUsageAmount >= remaining {
//			if err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponUsed); err != nil {
//				return err
//			}
//
//			currentUsageAmount = remaining
//		}
//
//		couponUsage := CouponUsage{
//			End:      time.Now().UTC(),
//			Amount:   currentUsageAmount,
//			CouponID: coupon.ID,
//		}
//
//		if err = service.db.Coupons().AddUsage(ctx, couponUsage); err != nil {
//			return err
//		}
//	}
//
//	return nil
//}

// updateTransactionsLoop updates all pending transactions in a loop.
func (service *Service) updateTransactionsLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	txsPage, err := service.db.Transactions().ListPending(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	if err := service.updateTransactions(ctx, txsPage.IDList()); err != nil {
		return err
	}

	for txsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		txsPage, err = service.db.Transactions().ListPending(ctx, txsPage.NextOffset, limit, before)
		if err != nil {
			return err
		}

		if err := service.updateTransactions(ctx, txsPage.IDList()); err != nil {
			return err
		}
	}

	return nil
}

// updateTransactions updates statuses and received amount for given transactions.
func (service *Service) updateTransactions(ctx context.Context, ids coinpayments.TransactionIDList) (err error) {
	defer mon.Task()(&ctx, ids)(&err)

	if len(ids) == 0 {
		service.log.Debug("no transactions found, skipping update")
		return nil
	}

	infos, err := service.coinPayments.Transactions().ListInfos(ctx, ids)
	if err != nil {
		return err
	}

	var updates []TransactionUpdate
	var applies coinpayments.TransactionIDList

	for id, info := range infos {
		updates = append(updates,
			TransactionUpdate{
				TransactionID: id,
				Status:        info.Status,
				Received:      info.Received,
			},
		)

		// moment of transition to received state, which indicates
		// that customer funds were accepted, so we can apply this
		// amount to customer balance. So we create intent to update
		// customer balance in the future.
		if info.Status == coinpayments.StatusReceived {
			applies = append(applies, id)
		}
	}

	return service.db.Transactions().Update(ctx, updates, applies)
}

// applyAccountBalanceLoop fetches all unapplied transaction in a loop, applying transaction
// received amount to stripe customer balance.
func (service *Service) updateAccountBalanceLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	txsPage, err := service.db.Transactions().ListUnapplied(ctx, 0, limit, before)
	if err != nil {
		return err
	}

	for _, tx := range txsPage.Transactions {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.applyTransactionBalance(ctx, tx); err != nil {
			return err
		}
	}

	for txsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		txsPage, err := service.db.Transactions().ListUnapplied(ctx, txsPage.NextOffset, limit, before)
		if err != nil {
			return err
		}

		for _, tx := range txsPage.Transactions {
			if err = ctx.Err(); err != nil {
				return err
			}

			if err = service.applyTransactionBalance(ctx, tx); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyTransactionBalance applies transaction received amount to stripe customer balance.
func (service *Service) applyTransactionBalance(ctx context.Context, tx Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	cusID, err := service.db.Customers().GetCustomerID(ctx, tx.AccountID)
	if err != nil {
		return err
	}

	rate, err := service.db.Transactions().GetLockedRate(ctx, tx.ID)
	if err != nil {
		return err
	}

	if err = service.db.Transactions().Consume(ctx, tx.ID); err != nil {
		return err
	}

	cents := convertToCents(rate, &tx.Received)

	params := &stripe.CustomerBalanceTransactionParams{
		Amount:      stripe.Int64(-cents),
		Customer:    stripe.String(cusID),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Description: stripe.String("storj token deposit"),
	}

	params.AddMetadata("txID", tx.ID.String())

	// TODO: 0 amount will return an error, how to handle that?
	_, err = service.stripeClient.CustomerBalanceTransactions.New(params)
	return err
}

// UpdateRates fetches new rates and updates service rate cache.
func (service *Service) UpdateRates(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	rates, err := service.coinPayments.ConversionRates().Get(ctx)

	service.mu.Lock()
	defer service.mu.Unlock()

	service.rates = rates
	service.ratesErr = err

	return err
}

// GetRate returns conversion rate for specified currencies.
func (service *Service) GetRate(ctx context.Context, curr1, curr2 coinpayments.Currency) (_ *big.Float, err error) {
	defer mon.Task()(&ctx)(&err)

	service.mu.Lock()
	defer service.mu.Unlock()

	if service.ratesErr != nil {
		return nil, Error.Wrap(err)
	}

	info1, ok := service.rates[curr1]
	if !ok {
		return nil, Error.New("no rate for currency %s", curr1)
	}
	info2, ok := service.rates[curr2]
	if !ok {
		return nil, Error.New("no rate for currency %s", curr2)
	}

	return new(big.Float).Quo(&info1.RateBTC, &info2.RateBTC), nil
}

// PrepareInvoiceProjectRecords iterates through all projects and creates invoice records if
// none exists.
func (service *Service) PrepareInvoiceProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25

	now := time.Now().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("prepare is for past periods only")
	}

	projsPage, err := service.projectsDB.List(ctx, 0, limit, end)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
		return Error.Wrap(err)
	}

	for projsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		projsPage, err = service.projectsDB.List(ctx, projsPage.NextOffset, limit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.createProjectRecords(ctx, projsPage.Projects, start, end); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, projects []console.Project, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if err == ErrProjectRecordExists {
				continue
			}

			return err
		}

		// TODO: account for usage data.
		records = append(records,
			CreateProjectRecord{
				ProjectID: project.ID,
				Storage:   0,
				Egress:    0,
				Objects:   0,
			},
		)
	}

	return service.db.ProjectRecords().Create(ctx, records, start, end)
}

// InvoiceApplyProjectRecords iterates through unapplied invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyProjectRecords(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	recordsPage, err := service.db.ProjectRecords().ListUnapplied(ctx, 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
		return Error.Wrap(err)
	}

	for recordsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		recordsPage, err = service.db.ProjectRecords().ListUnapplied(ctx, recordsPage.NextOffset, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// applyProjectRecords applies invoice intents as invoice line items to stripe customer.
func (service *Service) applyProjectRecords(ctx context.Context, records []ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return err
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			return err
		}

		cusID, err := service.db.Customers().GetCustomerID(ctx, proj.OwnerID)
		if err != nil {
			if err == ErrNoCustomer {
				continue
			}

			return err
		}

		if err = service.createInvoiceItems(ctx, cusID, proj.Name, record); err != nil {
			return err
		}
	}

	return nil
}

// createInvoiceItems consumes invoice project record and creates invoice line items for stripe customer.
func (service *Service) createInvoiceItems(ctx context.Context, cusID, projName string, record ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
		return err
	}

	// TODO: add and apply pricing.
	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(0),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(cusID),
		Description: stripe.String(fmt.Sprintf("project %s", projName)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(record.PeriodEnd.Unix()),
			Start: stripe.Int64(record.PeriodStart.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", record.ProjectID.String())

	_, err = service.stripeClient.InvoiceItems.New(projectItem)
	return err
}

// InvoiceApplyCoupons iterates through all active coupons.
// TODO: current implementation could possibly charge some coupons twice
// TODO: in case when this method failed and we call it again.
// TODO: we should mark coupons as 'already charged for current billing period' somehow to prevent multiple charge
// TODO: during invoices generation.
func (service *Service) InvoiceApplyCoupons(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	// takes first coupon page
	couponPage, err := service.db.Coupons().ListPaged(ctx, 0, limit, before, payments.CouponActive)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applyCouponsDiscount(ctx, couponPage.Coupons); err != nil {
		return Error.Wrap(err)
	}

	// iterates by rest pages
	for couponPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		couponPage, err = service.db.Coupons().ListPaged(ctx, couponPage.NextOffset, limit, before, payments.CouponActive)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applyCouponsDiscount(ctx, couponPage.Coupons); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// applyCouponsDiscount iterates through all coupons, gets total usage for this coupon and creates Invoice coupon item.
func (service *Service) applyCouponsDiscount(ctx context.Context, coupons []payments.Coupon) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, coupon := range coupons {
		customerID, err := service.db.Customers().GetCustomerID(ctx, coupon.UserID)
		if err != nil {
			if err != ErrNoCustomer {
				return err
			}

			service.log.Error(
				fmt.Sprintf("Could not apply coupon for user %s", coupon.UserID.String()),
				zap.Error(Error.Wrap(err)),
			)

			continue
		}

		amountToCharge, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
		if err != nil {
			return err
		}

		intervalEnd, err := service.db.Coupons().GetLatest(ctx, coupon.ID)
		if err != nil {
			return err
		}

		err = service.createInvoiceCouponItem(ctx, customerID, coupon, amountToCharge, intervalEnd)
		if err != nil {
			return err
		}
	}

	return nil
}

// createInvoiceCouponItem creates new Invoice item for specified coupon.
func (service *Service) createInvoiceCouponItem(ctx context.Context, customerID string, coupon payments.Coupon, amountToCharge int64, intervalEnd time.Time) (err error) {
	defer mon.Task()(&ctx, customerID, coupon)(&err)

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(-amountToCharge),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(customerID),
		Description: stripe.String(fmt.Sprintf("Discount from coupon: %s", coupon.Description)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(intervalEnd.Unix()),
			Start: stripe.Int64(coupon.Created.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", coupon.ProjectID.String())
	projectItem.AddMetadata("couponID", coupon.ID.String())

	_, err = service.stripeClient.InvoiceItems.New(projectItem)

	// TODO: do smth with coupon
	return err
}

// CreateInvoices lists through all customers and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now()

	cusPage, err := service.db.Customers().List(ctx, 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, cus := range cusPage.Customers {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		if err = service.createInvoice(ctx, cus.ID); err != nil {
			return Error.Wrap(err)
		}
	}

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		cusPage, err = service.db.Customers().List(ctx, cusPage.NextOffset, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		for _, cus := range cusPage.Customers {
			if err = ctx.Err(); err != nil {
				return Error.Wrap(err)
			}

			if err = service.createInvoice(ctx, cus.ID); err != nil {
				return Error.Wrap(err)
			}
		}
	}

	return nil
}

// createInvoice creates invoice for stripe customer. Returns nil error if there are no
// pending invoice line items for customer.
func (service *Service) createInvoice(ctx context.Context, cusID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = service.stripeClient.Invoices.New(
		&stripe.InvoiceParams{
			Customer:    stripe.String(cusID),
			AutoAdvance: stripe.Bool(true),
		},
	)

	if err != nil {
		if stripeErr, ok := err.(*stripe.Error); ok {
			switch stripeErr.Code {
			case stripe.ErrorCodeInvoiceNoCustomerLineItems:
				return nil
			default:
				return err
			}
		}
	}

	return nil
}
