// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
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

	ByteHourCents   decimal.Decimal
	EgressByteCents decimal.Decimal
	ObjectHourCents decimal.Decimal
	// BonusRate amount of percents
	BonusRate int64

	mu       sync.Mutex
	rates    coinpayments.CurrencyRateInfos
	ratesErr error
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, config Config, db DB, projectsDB console.Projects, usageDB accounting.ProjectAccounting, storageTBPrice, egressTBPrice, objectPrice string, bonusRate int64) (*Service, error) {
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

	tbMonthDollars, err := decimal.NewFromString(storageTBPrice)
	if err != nil {
		return nil, err
	}
	egressTBDollars, err := decimal.NewFromString(egressTBPrice)
	if err != nil {
		return nil, err
	}
	objectMonthDollars, err := decimal.NewFromString(objectPrice)
	if err != nil {
		return nil, err
	}

	// change the precision from dollars to cents
	tbMonthCents := tbMonthDollars.Shift(2)
	egressTBCents := egressTBDollars.Shift(2)
	objectHourCents := objectMonthDollars.Shift(2)

	// get per hour prices from storage and objects
	hoursPerMonth := decimal.New(30*24, 0)

	tbHourCents := tbMonthCents.Div(hoursPerMonth)
	objectHourCents = objectHourCents.Div(hoursPerMonth)

	// convert tb to bytes for storage and egress
	byteHourCents := tbHourCents.Div(decimal.New(1000000000000, 0))
	egressByteCents := egressTBCents.Div(decimal.New(1000000000000, 0))

	return &Service{
		log:             log,
		db:              db,
		projectsDB:      projectsDB,
		usageDB:         usageDB,
		stripeClient:    stripeClient,
		coinPayments:    coinPaymentsClient,
		ByteHourCents:   byteHourCents,
		EgressByteCents: egressByteCents,
		ObjectHourCents: objectHourCents,
		BonusRate:       bonusRate,
	}, nil
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

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
func (service *Service) updateTransactions(ctx context.Context, ids TransactionAndUserList) (err error) {
	defer mon.Task()(&ctx, ids)(&err)

	if len(ids) == 0 {
		service.log.Debug("no transactions found, skipping update")
		return nil
	}

	infos, err := service.coinPayments.Transactions().ListInfos(ctx, ids.IDList())
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

		// moment of transition to completed state, which indicates
		// that customer funds were accepted and transferred to our
		// account, so we can apply this amount to customer balance.
		// Therefore, create intent to update customer balance in the future.
		if info.Status == coinpayments.StatusCompleted {
			applies = append(applies, id)
		}

		userID := ids[id]

		rate, err := service.db.Transactions().GetLockedRate(ctx, id)
		if err != nil {
			service.log.Error(fmt.Sprintf("could not add promotional coupon for user %s", userID.String()), zap.Error(err))
			continue
		}

		cents := convertToCents(rate, &info.Received)

		if cents >= 5000 {
			err = service.Accounts().Coupons().AddPromotionalCoupon(ctx, userID, 2, 5500, memory.TB)
			if err != nil {
				service.log.Error(fmt.Sprintf("could not add promotional coupon for user %s", userID.String()), zap.Error(err))
				continue
			}
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
	if err != nil {
		return err
	}

	credit := payments.Credit{
		UserID:        tx.AccountID,
		Amount:        cents / 100 * service.BonusRate,
		TransactionID: tx.ID,
	}

	return service.db.Credits().InsertCredit(ctx, credit)
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
	var usages []CouponUsage
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

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return err
		}

		// TODO: account for usage data.
		records = append(records,
			CreateProjectRecord{
				ProjectID: project.ID,
				Storage:   usage.Storage,
				Egress:    usage.Egress,
				Objects:   usage.ObjectCount,
			},
		)

		coupons, err := service.db.Coupons().ListByProjectID(ctx, project.ID)
		if err != nil {
			return err
		}

		currentUsagePrice := service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount).TotalInt64()

		amountToChargeFromCoupon := currentUsagePrice

		// TODO: only for 1 coupon per project
		for _, coupon := range coupons {
			if coupon.IsExpired() {
				if err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponExpired); err != nil {
					return err
				}

				continue
			}

			alreadyChargedAmount, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
			if err != nil {
				return err
			}
			remaining := coupon.Amount - alreadyChargedAmount

			if amountToChargeFromCoupon >= remaining {
				amountToChargeFromCoupon = remaining
			}

			usages = append(usages, CouponUsage{
				Period:   start,
				Amount:   amountToChargeFromCoupon,
				Status:   CouponUsageStatusUnapplied,
				CouponID: coupon.ID,
			})
		}
	}

	return service.db.ProjectRecords().Create(ctx, records, usages, start, end)
}

// InvoiceApplyProjectRecords iterates through unapplied invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyProjectRecords(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now().UTC()

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

	projectPrice := service.calculateProjectUsagePrice(record.Egress, record.Storage, record.Objects)

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(projectPrice.TotalInt64()),
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

// InvoiceApplyCoupons iterates through unapplied project coupons and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyCoupons(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now().UTC()

	usagePage, err := service.db.Coupons().ListUnapplied(ctx, 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applyCoupons(ctx, usagePage.Usages); err != nil {
		return Error.Wrap(err)
	}

	for usagePage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		usagePage, err = service.db.Coupons().ListUnapplied(ctx, usagePage.NextOffset, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applyCoupons(ctx, usagePage.Usages); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// applyCoupons applies concrete coupon usage as invoice line item.
func (service *Service) applyCoupons(ctx context.Context, usages []CouponUsage) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, usage := range usages {
		if err = ctx.Err(); err != nil {
			return err
		}

		coupon, err := service.db.Coupons().Get(ctx, usage.CouponID)
		if err != nil {
			return err
		}

		customerID, err := service.db.Customers().GetCustomerID(ctx, coupon.UserID)
		if err != nil {
			if err == ErrNoCustomer {
				continue
			}

			return err
		}

		if err = service.createInvoiceCouponItems(ctx, coupon, usage, customerID); err != nil {
			return err
		}
	}

	return nil
}

// createInvoiceItems consumes invoice project record and creates invoice line items for stripe customer.
func (service *Service) createInvoiceCouponItems(ctx context.Context, coupon payments.Coupon, usage CouponUsage, customerID string) (err error) {
	defer mon.Task()(&ctx, customerID, coupon)(&err)

	err = service.db.Coupons().ApplyUsage(ctx, usage.CouponID, usage.Period)
	if err != nil {
		return err
	}

	totalUsage, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
	if err != nil {
		return err
	}
	if totalUsage == coupon.Amount {
		err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponUsed)
		if err != nil {
			return err
		}
	}

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(-usage.Amount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(customerID),
		Description: stripe.String(fmt.Sprintf("Discount from coupon: %s", coupon.Description)),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(usage.Period.AddDate(0, 1, 0).Unix()),
			Start: stripe.Int64(usage.Period.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", coupon.ProjectID.String())
	projectItem.AddMetadata("couponID", coupon.ID.String())

	_, err = service.stripeClient.InvoiceItems.New(projectItem)

	return err
}

// InvoiceApplyCredits iterates through credits with status false of project and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyCredits(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	const limit = 25
	before := time.Now().UTC()

	spendingsPage, err := service.db.Credits().ListCreditsSpendingsPaged(ctx, int(CreditsSpendingStatusUnapplied), 0, limit, before)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applySpendings(ctx, spendingsPage.Spendings); err != nil {
		return Error.Wrap(err)
	}

	for spendingsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		spendingsPage, err = service.db.Credits().ListCreditsSpendingsPaged(ctx, int(CreditsSpendingStatusUnapplied), spendingsPage.NextOffset, limit, before)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applySpendings(ctx, spendingsPage.Spendings); err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// applyCredits applies concrete spending as invoice line item.
func (service *Service) applySpendings(ctx context.Context, spendings []CreditsSpending) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, spending := range spendings {
		if err = ctx.Err(); err != nil {
			return err
		}

		if err = service.createInvoiceCreditItem(ctx, spending); err != nil {
			return err
		}
	}

	return nil
}

// createInvoiceItems consumes invoice project record and creates invoice line items for stripe customer.
func (service *Service) createInvoiceCreditItem(ctx context.Context, spending CreditsSpending) (err error) {
	defer mon.Task()(&ctx, spending)(&err)

	err = service.db.Credits().ApplyCreditsSpending(ctx, spending.ID, int(CreditsSpendingStatusApplied))
	if err != nil {
		return err
	}
	customerID, err := service.db.Customers().GetCustomerID(ctx, spending.UserID)

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(-spending.Amount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(customerID),
		Description: stripe.String(fmt.Sprintf("Discount from credits")),
		Period: &stripe.InvoiceItemPeriodParams{
			End:   stripe.Int64(spending.Created.AddDate(0, 1, 0).Unix()),
			Start: stripe.Int64(spending.Created.Unix()),
		},
	}

	projectItem.AddMetadata("projectID", spending.ProjectID.String())
	projectItem.AddMetadata("userID", spending.UserID.String())

	_, err = service.stripeClient.InvoiceItems.New(projectItem)

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

// projectUsagePrice represents pricing for project usage.
type projectUsagePrice struct {
	Storage decimal.Decimal
	Egress  decimal.Decimal
	Objects decimal.Decimal
}

// Total returns project usage price total.
func (price projectUsagePrice) Total() decimal.Decimal {
	return price.Storage.Add(price.Egress).Add(price.Objects)
}

// Total returns project usage price total.
func (price projectUsagePrice) TotalInt64() int64 {
	return price.Storage.Add(price.Egress).Add(price.Objects).IntPart()
}

// calculateProjectUsagePrice calculate project usage price.
func (service *Service) calculateProjectUsagePrice(egress int64, storage, objects float64) projectUsagePrice {
	return projectUsagePrice{
		Storage: service.ByteHourCents.Mul(decimal.NewFromFloat(storage)),
		Egress:  service.EgressByteCents.Mul(decimal.New(egress, 0)),
		Objects: service.ObjectHourCents.Mul(decimal.NewFromFloat(objects)),
	}
}
