// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go/v72"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/monetary"
)

var (
	// Error defines stripecoinpayments service error.
	Error = errs.Class("stripecoinpayments service")

	mon = monkit.Package()
)

// hoursPerMonth is the number of months in a billing month. For the purpose of billing, the billing month is always 30 days.
const hoursPerMonth = 24 * 30

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey              string        `help:"stripe API secret key" default:""`
	StripePublicKey              string        `help:"stripe API public key" default:""`
	StripeFreeTierCouponID       string        `help:"stripe free tier coupon ID" default:""`
	CoinpaymentsPublicKey        string        `help:"coinpayments API public key" default:""`
	CoinpaymentsPrivateKey       string        `help:"coinpayments API private key key" default:""`
	TransactionUpdateInterval    time.Duration `help:"amount of time we wait before running next transaction update loop" default:"2m" testDefault:"$TESTINTERVAL"`
	AccountBalanceUpdateInterval time.Duration `help:"amount of time we wait before running next account balance update loop" default:"2m" testDefault:"$TESTINTERVAL"`
	ConversionRatesCycleInterval time.Duration `help:"amount of time we wait before running next conversion rates update loop" default:"10m" testDefault:"$TESTINTERVAL"`
	AutoAdvance                  bool          `help:"toogle autoadvance feature for invoice creation" default:"false"`
	ListingLimit                 int           `help:"sets the maximum amount of items before we start paging on requests" default:"100" hidden:"true"`

	// temporary! remove after all gob-encoded big.Float values are out of all satellite DBs.
	GobFloatMigrationBatchInterval time.Duration `help:"amount of time to wait between gob-encoded big.Float database migration batches" default:"1m" testDefault:"$TESTINTERVAL" hidden:"true"`
	GobFloatMigrationBatchSize     int           `help:"number of rows with gob-encoded big.Float values to migrate at once" default:"100" testDefault:"10" hidden:"true"`
}

// Service is an implementation for payment service via Stripe and Coinpayments.
//
// architecture: Service
type Service struct {
	log          *zap.Logger
	db           DB
	projectsDB   console.Projects
	usageDB      accounting.ProjectAccounting
	stripeClient StripeClient
	coinPayments *coinpayments.Client

	StorageMBMonthPriceCents decimal.Decimal
	EgressMBPriceCents       decimal.Decimal
	SegmentMonthPriceCents   decimal.Decimal
	// BonusRate amount of percents
	BonusRate int64
	// Coupon Values
	StripeFreeTierCouponID string

	// Stripe Extended Features
	AutoAdvance bool

	mu       sync.Mutex
	rates    coinpayments.CurrencyRateInfos
	ratesErr error

	listingLimit int
	nowFn        func() time.Time
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, stripeClient StripeClient, config Config, db DB, projectsDB console.Projects, usageDB accounting.ProjectAccounting, storageTBPrice, egressTBPrice, segmentPrice string, bonusRate int64) (*Service, error) {

	coinPaymentsClient := coinpayments.NewClient(
		coinpayments.Credentials{
			PublicKey:  config.CoinpaymentsPublicKey,
			PrivateKey: config.CoinpaymentsPrivateKey,
		},
	)

	storageTBMonthDollars, err := decimal.NewFromString(storageTBPrice)
	if err != nil {
		return nil, err
	}
	egressTBDollars, err := decimal.NewFromString(egressTBPrice)
	if err != nil {
		return nil, err
	}
	segmentMonthDollars, err := decimal.NewFromString(segmentPrice)
	if err != nil {
		return nil, err
	}

	// change the precision from TB dollars to MB cents
	storageMBMonthPriceCents := storageTBMonthDollars.Shift(-6).Shift(2)
	egressMBPriceCents := egressTBDollars.Shift(-6).Shift(2)
	segmentMonthPriceCents := segmentMonthDollars.Shift(2)

	return &Service{
		log:                      log,
		db:                       db,
		projectsDB:               projectsDB,
		usageDB:                  usageDB,
		stripeClient:             stripeClient,
		coinPayments:             coinPaymentsClient,
		StorageMBMonthPriceCents: storageMBMonthPriceCents,
		EgressMBPriceCents:       egressMBPriceCents,
		SegmentMonthPriceCents:   segmentMonthPriceCents,
		BonusRate:                bonusRate,
		StripeFreeTierCouponID:   config.StripeFreeTierCouponID,
		AutoAdvance:              config.AutoAdvance,
		listingLimit:             config.ListingLimit,
		nowFn:                    time.Now,
	}, nil
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

// updateTransactionsLoop updates all pending transactions in a loop.
func (service *Service) updateTransactionsLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	before := service.nowFn()

	txsPage, err := service.db.Transactions().ListPending(ctx, 0, service.listingLimit, before)
	if err != nil {
		return err
	}

	if err := service.updateTransactions(ctx, txsPage.IDList(), txsPage.CreationTimes()); err != nil {
		return err
	}

	for txsPage.Next {
		if err = ctx.Err(); err != nil {
			return err
		}

		txsPage, err = service.db.Transactions().ListPending(ctx, txsPage.NextOffset, service.listingLimit, before)
		if err != nil {
			return err
		}

		if err := service.updateTransactions(ctx, txsPage.IDList(), txsPage.CreationTimes()); err != nil {
			return err
		}
	}

	return nil
}

// updateTransactions updates statuses and received amount for given transactions.
func (service *Service) updateTransactions(ctx context.Context, ids TransactionAndUserList, creationTimes map[coinpayments.TransactionID]time.Time) (err error) {
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
		service.log.Debug("Coinpayments results: ", zap.String("status", info.Status.String()), zap.String("id", id.String()))
		updates = append(updates,
			TransactionUpdate{
				TransactionID: id,
				Status:        info.Status,
				Received:      monetary.AmountFromDecimal(info.Received, monetary.StorjToken),
			},
		)

		// moment of CoinPayments receives funds, not when STORJ does
		// this was a business decision to not wait until StatusCompleted
		if info.Status >= coinpayments.StatusReceived {
			// monkit currently does not have a DurationVal
			mon.IntVal("coinpayment_duration").Observe(int64(time.Since(creationTimes[id])))
			applies = append(applies, id)
		}
	}

	return service.db.Transactions().Update(ctx, updates, applies)
}

// applyAccountBalanceLoop fetches all unapplied transaction in a loop, applying transaction
// received amount to stripe customer balance.
func (service *Service) updateAccountBalanceLoop(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	before := service.nowFn()

	txsPage, err := service.db.Transactions().ListUnapplied(ctx, 0, service.listingLimit, before)
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

		txsPage, err = service.db.Transactions().ListUnapplied(ctx, txsPage.NextOffset, service.listingLimit, before)
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

	cents := convertToCents(rate, tx.Received)

	if cents <= 0 {
		service.log.Warn("Trying to deposit non-positive amount.",
			zap.Int64("USD cents", cents),
			zap.Stringer("Transaction ID", tx.ID),
			zap.Stringer("User ID", tx.AccountID),
		)
		return service.db.Transactions().Consume(ctx, tx.ID)
	}

	// Check for balance transactions created from previous failed attempt
	var depositDone, bonusDone bool
	it := service.stripeClient.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{Customer: stripe.String(cusID)})
	for it.Next() {
		cbt := it.CustomerBalanceTransaction()

		if cbt.Type != stripe.CustomerBalanceTransactionTypeAdjustment {
			continue
		}

		txID, ok := cbt.Metadata["txID"]
		if !ok {
			continue
		}
		if txID != tx.ID.String() {
			continue
		}

		switch cbt.Description {
		case StripeDepositTransactionDescription:
			depositDone = true
		case StripeDepositBonusTransactionDescription:
			bonusDone = true
		}
	}

	// The first balance transaction is for the actual deposit
	if !depositDone {
		params := &stripe.CustomerBalanceTransactionParams{
			Amount:      stripe.Int64(-cents),
			Customer:    stripe.String(cusID),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Description: stripe.String(StripeDepositTransactionDescription),
		}
		params.AddMetadata("txID", tx.ID.String())
		params.AddMetadata("storj_amount", tx.Amount.AsDecimal().String())
		params.AddMetadata("storj_usd_rate", rate.String())
		_, err = service.stripeClient.CustomerBalanceTransactions().New(params)
		if err != nil {
			return err
		}
	}

	// The second balance transaction for the bonus
	if !bonusDone {
		params := &stripe.CustomerBalanceTransactionParams{
			Amount:      stripe.Int64(-cents * service.BonusRate / 100),
			Customer:    stripe.String(cusID),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Description: stripe.String(StripeDepositBonusTransactionDescription),
		}
		params.AddMetadata("txID", tx.ID.String())
		params.AddMetadata("percentage", strconv.Itoa(int(service.BonusRate)))
		_, err = service.stripeClient.CustomerBalanceTransactions().New(params)
		if err != nil {
			return err
		}
	}

	return service.db.Transactions().Consume(ctx, tx.ID)
}

// UpdateRates fetches new rates and updates service rate cache.
func (service *Service) UpdateRates(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	rates, err := service.coinPayments.ConversionRates().Get(ctx)
	if coinpayments.ErrMissingPublicKey.Has(err) {
		rates = coinpayments.CurrencyRateInfos{}
		err = nil

		service.log.Info("Coinpayment client is missing public key")
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	service.rates = rates
	service.ratesErr = err

	return err
}

// GetRate returns conversion rate for specified currencies.
func (service *Service) GetRate(ctx context.Context, curr1, curr2 *monetary.Currency) (_ decimal.Decimal, err error) {
	defer mon.Task()(&ctx)(&err)

	service.mu.Lock()
	defer service.mu.Unlock()

	if service.ratesErr != nil {
		return decimal.Decimal{}, Error.Wrap(err)
	}

	info1, ok := service.rates.ForCurrency(curr1)
	if !ok {
		return decimal.Decimal{}, Error.New("no rate for currency %s", curr1.Name())
	}
	info2, ok := service.rates.ForCurrency(curr2)
	if !ok {
		return decimal.Decimal{}, Error.New("no rate for currency %s", curr2.Name())
	}

	return info1.RateBTC.Div(info2.RateBTC), nil
}

// PrepareInvoiceProjectRecords iterates through all projects and creates invoice records if none exist.
func (service *Service) PrepareInvoiceProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var numberOfCustomers, numberOfRecords int
	customersPage, err := service.db.Customers().List(ctx, 0, service.listingLimit, end)
	if err != nil {
		return Error.Wrap(err)
	}
	numberOfCustomers += len(customersPage.Customers)

	records, err := service.processCustomers(ctx, customersPage.Customers, start, end)
	if err != nil {
		return Error.Wrap(err)
	}
	numberOfRecords += records

	for customersPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		customersPage, err = service.db.Customers().List(ctx, customersPage.NextOffset, service.listingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		records, err := service.processCustomers(ctx, customersPage.Customers, start, end)
		if err != nil {
			return Error.Wrap(err)
		}
		numberOfRecords += records
	}

	service.log.Info("Number of processed entries.", zap.Int("Customers", numberOfCustomers), zap.Int("Projects", numberOfRecords))
	return nil
}

func (service *Service) processCustomers(ctx context.Context, customers []Customer, start, end time.Time) (int, error) {
	var allRecords []CreateProjectRecord
	for _, customer := range customers {
		projects, err := service.projectsDB.GetOwn(ctx, customer.UserID)
		if err != nil {
			return 0, err
		}

		records, err := service.createProjectRecords(ctx, customer.ID, projects, start, end)
		if err != nil {
			return 0, err
		}

		allRecords = append(allRecords, records...)
	}

	return len(allRecords), service.db.ProjectRecords().Create(ctx, allRecords, start, end)
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, customerID string, projects []console.Project, start, end time.Time) (_ []CreateProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if errors.Is(err, ErrProjectRecordExists) {
				service.log.Warn("Record for this project already exists.", zap.String("Customer ID", customerID), zap.String("Project ID", project.ID.String()))
				continue
			}

			return nil, err
		}

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return nil, err
		}

		// TODO: account for usage data.
		records = append(records,
			CreateProjectRecord{
				ProjectID: project.ID,
				Storage:   usage.Storage,
				Egress:    usage.Egress,
				Segments:  usage.SegmentCount,
			},
		)
	}

	return records, nil
}

// InvoiceApplyProjectRecords iterates through unapplied invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	projectRecords := 0
	recordsPage, err := service.db.ProjectRecords().ListUnapplied(ctx, 0, service.listingLimit, start, end)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
		return Error.Wrap(err)
	}

	projectRecords += len(recordsPage.Records)

	for recordsPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		// we are always starting from offset 0 because applyProjectRecords is changing project record state to applied
		recordsPage, err = service.db.ProjectRecords().ListUnapplied(ctx, 0, service.listingLimit, start, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applyProjectRecords(ctx, recordsPage.Records); err != nil {
			return Error.Wrap(err)
		}

		projectRecords += len(recordsPage.Records)
	}

	service.log.Info("Number of processed project records.", zap.Int("Project Records", projectRecords))
	return nil
}

// applyProjectRecords applies invoice intents as invoice line items to stripe customer.
func (service *Service) applyProjectRecords(ctx context.Context, records []ProjectRecord) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return errs.Wrap(err)
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			// This should never happen, but be sure to log info to further troubleshoot before exiting.
			service.log.Error("project ID for corresponding project record not found", zap.Stringer("Record ID", record.ID), zap.Stringer("Project ID", record.ProjectID))
			return errs.Wrap(err)
		}

		cusID, err := service.db.Customers().GetCustomerID(ctx, proj.OwnerID)
		if err != nil {
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("Stripe customer does not exist for project owner.", zap.Stringer("Owner ID", proj.OwnerID), zap.Stringer("Project ID", proj.ID))
				continue
			}

			return errs.Wrap(err)
		}

		if err = service.createInvoiceItems(ctx, cusID, proj.Name, record); err != nil {
			return errs.Wrap(err)
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

	items := service.InvoiceItemsFromProjectRecord(projName, record)
	for _, item := range items {
		item.Currency = stripe.String(string(stripe.CurrencyUSD))
		item.Customer = stripe.String(cusID)
		item.AddMetadata("projectID", record.ProjectID.String())

		_, err = service.stripeClient.InvoiceItems().New(item)
		if err != nil {
			return err
		}
	}

	return nil
}

// InvoiceItemsFromProjectRecord calculates Stripe invoice item from project record.
func (service *Service) InvoiceItemsFromProjectRecord(projName string, record ProjectRecord) (result []*stripe.InvoiceItemParams) {
	projectItem := &stripe.InvoiceItemParams{}
	projectItem.Description = stripe.String(fmt.Sprintf("Project %s - Segment Storage (MB-Month)", projName))
	projectItem.Quantity = stripe.Int64(storageMBMonthDecimal(record.Storage).IntPart())
	storagePrice, _ := service.StorageMBMonthPriceCents.Float64()
	projectItem.UnitAmountDecimal = stripe.Float64(storagePrice)
	result = append(result, projectItem)

	projectItem = &stripe.InvoiceItemParams{}
	projectItem.Description = stripe.String(fmt.Sprintf("Project %s - Egress Bandwidth (MB)", projName))
	projectItem.Quantity = stripe.Int64(egressMBDecimal(record.Egress).IntPart())
	egressPrice, _ := service.EgressMBPriceCents.Float64()
	projectItem.UnitAmountDecimal = stripe.Float64(egressPrice)
	result = append(result, projectItem)

	projectItem = &stripe.InvoiceItemParams{}
	projectItem.Description = stripe.String(fmt.Sprintf("Project %s - Segment Fee (Segment-Month)", projName))
	projectItem.Quantity = stripe.Int64(segmentMonthDecimal(record.Segments).IntPart())
	segmentPrice, _ := service.SegmentMonthPriceCents.Float64()
	projectItem.UnitAmountDecimal = stripe.Float64(segmentPrice)
	result = append(result, projectItem)
	service.log.Info("invoice items", zap.Any("result", result))

	return result
}

// ApplyFreeTierCoupons iterates through all customers in Stripe. For each customer,
// if that customer does not currently have a Stripe coupon, the free tier Stripe coupon
// is applied.
func (service *Service) ApplyFreeTierCoupons(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	customers := service.db.Customers()

	appliedCoupons := 0
	failedUsers := []string{}
	morePages := true
	nextOffset := int64(0)
	listingLimit := 100
	end := time.Now()
	for morePages {
		customersPage, err := customers.List(ctx, nextOffset, listingLimit, end)
		if err != nil {
			return err
		}
		morePages = customersPage.Next
		nextOffset = customersPage.NextOffset

		for _, c := range customersPage.Customers {
			stripeCust, err := service.stripeClient.Customers().Get(c.ID, nil)
			if err != nil {
				service.log.Error("Failed to get customer", zap.Error(err))
				failedUsers = append(failedUsers, c.ID)
				continue
			}
			// if customer does not have a coupon, apply the free tier coupon
			if stripeCust.Discount == nil || stripeCust.Discount.Coupon == nil {
				params := &stripe.CustomerParams{
					Coupon: stripe.String(service.StripeFreeTierCouponID),
				}
				_, err := service.stripeClient.Customers().Update(c.ID, params)
				if err != nil {
					service.log.Error("Failed to update customer with free tier coupon", zap.Error(err))
					failedUsers = append(failedUsers, c.ID)
					continue
				}
				appliedCoupons++
			}
		}
	}

	if len(failedUsers) > 0 {
		service.log.Warn("Failed to get or apply free tier coupon to some customers:", zap.String("idlist", strings.Join(failedUsers, ", ")))
	}
	service.log.Info("Finished", zap.Int("number of coupons applied", appliedCoupons))

	return nil
}

// CreateInvoices lists through all customers and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	invoices := 0
	cusPage, err := service.db.Customers().List(ctx, 0, service.listingLimit, end)
	if err != nil {
		return Error.Wrap(err)
	}

	for _, cus := range cusPage.Customers {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		if err = service.createInvoice(ctx, cus.ID, start); err != nil {
			return Error.Wrap(err)
		}
	}

	invoices += len(cusPage.Customers)

	for cusPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		cusPage, err = service.db.Customers().List(ctx, cusPage.NextOffset, service.listingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		for _, cus := range cusPage.Customers {
			if err = ctx.Err(); err != nil {
				return Error.Wrap(err)
			}

			if err = service.createInvoice(ctx, cus.ID, start); err != nil {
				return Error.Wrap(err)
			}
		}

		invoices += len(cusPage.Customers)
	}

	service.log.Info("Number of created draft invoices.", zap.Int("Invoices", invoices))
	return nil
}

// createInvoice creates invoice for stripe customer. Returns nil error if there are no
// pending invoice line items for customer.
func (service *Service) createInvoice(ctx context.Context, cusID string, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	description := fmt.Sprintf("Storj DCS Cloud Storage for %s %d", period.Month(), period.Year())

	_, err = service.stripeClient.Invoices().New(
		&stripe.InvoiceParams{
			Customer:    stripe.String(cusID),
			AutoAdvance: stripe.Bool(service.AutoAdvance),
			Description: stripe.String(description),
		},
	)

	if err != nil {
		var stripErr *stripe.Error
		if errors.As(err, &stripErr) {
			if stripErr.Code == stripe.ErrorCodeInvoiceNoCustomerLineItems {
				return nil
			}
		}
		return err
	}

	return nil
}

// FinalizeInvoices sets autoadvance flag on all draft invoices currently available in stripe.
func (service *Service) FinalizeInvoices(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		Status: stripe.String("draft"),
	}

	invoicesIterator := service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		err := service.finalizeInvoice(ctx, stripeInvoice.ID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return Error.Wrap(invoicesIterator.Err())
}

func (service *Service) finalizeInvoice(ctx context.Context, invoiceID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceFinalizeParams{AutoAdvance: stripe.Bool(true)}
	_, err = service.stripeClient.Invoices().FinalizeInvoice(invoiceID, params)
	return err
}

// projectUsagePrice represents pricing for project usage.
type projectUsagePrice struct {
	Storage  decimal.Decimal
	Egress   decimal.Decimal
	Segments decimal.Decimal
}

// Total returns project usage price total.
func (price projectUsagePrice) Total() decimal.Decimal {
	return price.Storage.Add(price.Egress).Add(price.Segments)
}

// Total returns project usage price total.
func (price projectUsagePrice) TotalInt64() int64 {
	return price.Storage.Add(price.Egress).Add(price.Segments).IntPart()
}

// calculateProjectUsagePrice calculate project usage price.
func (service *Service) calculateProjectUsagePrice(egress int64, storage, segments float64) projectUsagePrice {
	return projectUsagePrice{
		Storage:  service.StorageMBMonthPriceCents.Mul(storageMBMonthDecimal(storage)).Round(0),
		Egress:   service.EgressMBPriceCents.Mul(egressMBDecimal(egress)).Round(0),
		Segments: service.SegmentMonthPriceCents.Mul(segmentMonthDecimal(segments)).Round(0),
	}
}

// SetNow allows tests to have the Service act as if the current time is whatever
// they want. This avoids races and sleeping, making tests more reliable and efficient.
func (service *Service) SetNow(now func() time.Time) {
	service.nowFn = now
}

// storageMBMonthDecimal converts storage usage from Byte-Hours to Megabyte-Months.
// The result is rounded to the nearest whole number, but returned as Decimal for convenience.
func storageMBMonthDecimal(storage float64) decimal.Decimal {
	return decimal.NewFromFloat(storage).Shift(-6).Div(decimal.NewFromInt(hoursPerMonth)).Round(0)
}

// egressMBDecimal converts egress usage from bytes to Megabytes
// The result is rounded to the nearest whole number, but returned as Decimal for convenience.
func egressMBDecimal(egress int64) decimal.Decimal {
	return decimal.NewFromInt(egress).Shift(-6).Round(0)
}

// segmentMonthDecimal converts segments usage from Segment-Hours to Segment-Months.
// The result is rounded to the nearest whole number, but returned as Decimal for convenience.
func segmentMonthDecimal(segments float64) decimal.Decimal {
	return decimal.NewFromFloat(segments).Div(decimal.NewFromInt(hoursPerMonth)).Round(0)
}
