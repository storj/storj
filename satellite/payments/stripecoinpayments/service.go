// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go"
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

// hoursPerMonth is the number of months in a billing month. For the purpose of billing, the billing month is always 30 days.
const hoursPerMonth = 24 * 30

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey              string        `help:"stripe API secret key" default:""`
	StripePublicKey              string        `help:"stripe API public key" default:""`
	CoinpaymentsPublicKey        string        `help:"coinpayments API public key" default:""`
	CoinpaymentsPrivateKey       string        `help:"coinpayments API private key key" default:""`
	TransactionUpdateInterval    time.Duration `help:"amount of time we wait before running next transaction update loop" default:"2m"`
	AccountBalanceUpdateInterval time.Duration `help:"amount of time we wait before running next account balance update loop" default:"2m"`
	ConversionRatesCycleInterval time.Duration `help:"amount of time we wait before running next conversion rates update loop" default:"10m"`
	AutoAdvance                  bool          `help:"toogle autoadvance feature for invoice creation" default:"false"`
	ListingLimit                 int           `help:"sets the maximum amount of items before we start paging on requests" default:"100" hidden:"true"`
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
	ObjectMonthPriceCents    decimal.Decimal
	// BonusRate amount of percents
	BonusRate int64
	// Coupon Values
	CouponValue        int64
	CouponDuration     int64
	CouponProjectLimit memory.Size
	// Minimum CoinPayment to create a coupon
	MinCoinPayment int64

	// Stripe Extended Features
	AutoAdvance bool

	mu       sync.Mutex
	rates    coinpayments.CurrencyRateInfos
	ratesErr error

	listingLimit      int
	nowFn             func() time.Time
	PaywallProportion float64
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, stripeClient StripeClient, config Config, db DB, projectsDB console.Projects, usageDB accounting.ProjectAccounting, storageTBPrice, egressTBPrice, objectPrice string, bonusRate, couponValue, couponDuration int64, couponProjectLimit memory.Size, minCoinPayment int64, paywallProportion float64) (*Service, error) {

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
	objectMonthDollars, err := decimal.NewFromString(objectPrice)
	if err != nil {
		return nil, err
	}

	// change the precision from TB dollars to MB cents
	storageMBMonthPriceCents := storageTBMonthDollars.Shift(-6).Shift(2)
	egressMBPriceCents := egressTBDollars.Shift(-6).Shift(2)
	objectMonthPriceCents := objectMonthDollars.Shift(2)

	return &Service{
		log:                      log,
		db:                       db,
		projectsDB:               projectsDB,
		usageDB:                  usageDB,
		stripeClient:             stripeClient,
		coinPayments:             coinPaymentsClient,
		StorageMBMonthPriceCents: storageMBMonthPriceCents,
		EgressMBPriceCents:       egressMBPriceCents,
		ObjectMonthPriceCents:    objectMonthPriceCents,
		BonusRate:                bonusRate,
		CouponValue:              couponValue,
		CouponDuration:           couponDuration,
		CouponProjectLimit:       couponProjectLimit,
		MinCoinPayment:           minCoinPayment,
		AutoAdvance:              config.AutoAdvance,
		listingLimit:             config.ListingLimit,
		nowFn:                    time.Now,
		PaywallProportion:        paywallProportion,
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
				Received:      info.Received,
			},
		)

		// moment of CoinPayments receives funds, not when STORJ does
		// this was a business decision to not wait until StatusCompleted
		if info.Status >= coinpayments.StatusReceived {
			// monkit currently does not have a DurationVal
			mon.IntVal("coinpayment_duration").Observe(int64(time.Since(creationTimes[id])))
			applies = append(applies, id)
		}

		userID := ids[id]

		if !service.Accounts().PaywallEnabled(userID) {
			continue
		}

		rate, err := service.db.Transactions().GetLockedRate(ctx, id)
		if err != nil {
			service.log.Error(fmt.Sprintf("could not add promotional coupon for user %s", userID.String()), zap.Error(err))
			continue
		}

		cents := convertToCents(rate, &info.Received)

		if cents >= service.MinCoinPayment {
			err = service.Accounts().Coupons().AddPromotionalCoupon(ctx, userID)
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

	cents := convertToCents(rate, &tx.Received)

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
		params.AddMetadata("storj_amount", tx.Amount.String())
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

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var numberOfCustomers, numberOfRecords, numberOfCouponsUsages int
	customersPage, err := service.db.Customers().List(ctx, 0, service.listingLimit, end)
	if err != nil {
		return Error.Wrap(err)
	}
	numberOfCustomers += len(customersPage.Customers)

	records, usages, err := service.processCustomers(ctx, customersPage.Customers, start, end)
	if err != nil {
		return Error.Wrap(err)
	}
	numberOfRecords += records
	numberOfCouponsUsages += usages

	for customersPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		customersPage, err = service.db.Customers().List(ctx, customersPage.NextOffset, service.listingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		records, usages, err := service.processCustomers(ctx, customersPage.Customers, start, end)
		if err != nil {
			return Error.Wrap(err)
		}
		numberOfRecords += records
		numberOfCouponsUsages += usages
	}

	service.log.Info("Number of processed entries.", zap.Int("Customers", numberOfCustomers), zap.Int("Projects", numberOfRecords), zap.Int("Coupons Usages", numberOfCouponsUsages))
	return nil
}

func (service *Service) processCustomers(ctx context.Context, customers []Customer, start, end time.Time) (int, int, error) {
	var allRecords []CreateProjectRecord
	var usages []CouponUsage
	for _, customer := range customers {
		projects, err := service.projectsDB.GetOwn(ctx, customer.UserID)
		if err != nil {
			return 0, 0, err
		}

		leftToCharge, records, err := service.createProjectRecords(ctx, customer.ID, projects, start, end)
		if err != nil {
			return 0, 0, err
		}

		allRecords = append(allRecords, records...)

		coupons, err := service.db.Coupons().ListByUserIDAndStatus(ctx, customer.UserID, payments.CouponActive)
		if err != nil {
			return 0, 0, err
		}

		// Apply any promotional credits (a.k.a. coupons) on the remainder.
		for _, coupon := range coupons {
			if coupon.Status == payments.CouponExpired {
				// this coupon has already been marked as expired.
				continue
			}

			if end.After(coupon.ExpirationDate()) {
				// this coupon is identified as expired for first time, mark it in the database
				if _, err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponExpired); err != nil {
					return 0, 0, err
				}
				continue
			}

			alreadyChargedAmount, err := service.db.Coupons().TotalUsage(ctx, coupon.ID)
			if err != nil {
				return 0, 0, err
			}
			remaining := coupon.Amount - alreadyChargedAmount

			amountToChargeFromCoupon := leftToCharge
			if amountToChargeFromCoupon >= remaining {
				amountToChargeFromCoupon = remaining
			}

			if amountToChargeFromCoupon > 0 {
				usages = append(usages, CouponUsage{
					Period:   start,
					Amount:   amountToChargeFromCoupon,
					Status:   CouponUsageStatusUnapplied,
					CouponID: coupon.ID,
				})

				leftToCharge -= amountToChargeFromCoupon
			}

			if amountToChargeFromCoupon < remaining && end.Equal(coupon.ExpirationDate()) {
				// the coupon was not fully spent, but this is the last month
				// it is valid for, so mark it as expired in database
				if _, err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponExpired); err != nil {
					return 0, 0, err
				}
			}
		}
	}

	return len(allRecords), len(usages), service.db.ProjectRecords().Create(ctx, allRecords, usages, start, end)
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, customerID string, projects []console.Project, start, end time.Time) (_ int64, _ []CreateProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	sumLeftToCharge := int64(0)
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return 0, nil, err
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if errors.Is(err, ErrProjectRecordExists) {
				service.log.Warn("Record for this project already exists.", zap.String("Customer ID", customerID), zap.String("Project ID", project.ID.String()))
				continue
			}

			return 0, nil, err
		}

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, start, end)
		if err != nil {
			return 0, nil, err
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

		leftToCharge := service.calculateProjectUsagePrice(usage.Egress, usage.Storage, usage.ObjectCount).TotalInt64()
		if leftToCharge == 0 {
			continue
		}

		// If there is a Stripe coupon applied for the project owner, apply its
		// discount first before applying other credits of this user. This
		// avoids the issue with negative totals in invoices.
		leftToCharge, err = service.discountedProjectUsagePrice(ctx, customerID, leftToCharge)
		if err != nil {
			return 0, nil, err
		}

		sumLeftToCharge += leftToCharge
	}

	return sumLeftToCharge, records, nil
}

// InvoiceApplyProjectRecords iterates through unapplied invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 0, 0, 0, 0, 0, time.UTC)

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
			return err
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			return err
		}

		cusID, err := service.db.Customers().GetCustomerID(ctx, proj.OwnerID)
		if err != nil {
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("Stripe customer does not exist for project owner.", zap.Stringer("Owner ID", proj.OwnerID), zap.Stringer("Project ID", proj.ID))
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
	projectItem.Description = stripe.String(fmt.Sprintf("Project %s - Object Storage (MB-Month)", projName))
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
	projectItem.Description = stripe.String(fmt.Sprintf("Project %s - Object Fee (Object-Month)", projName))
	projectItem.Quantity = stripe.Int64(objectMonthDecimal(record.Objects).IntPart())
	objectPrice, _ := service.ObjectMonthPriceCents.Float64()
	projectItem.UnitAmountDecimal = stripe.Float64(objectPrice)
	result = append(result, projectItem)

	return result
}

// InvoiceApplyCoupons iterates through unapplied project coupons and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyCoupons(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	couponsUsages := 0
	usagePage, err := service.db.Coupons().ListUnapplied(ctx, 0, service.listingLimit, start)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = service.applyCoupons(ctx, usagePage.Usages); err != nil {
		return Error.Wrap(err)
	}

	couponsUsages += len(usagePage.Usages)

	for usagePage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		// we are always starting from offset 0 because applyCoupons is changing coupon usage state to applied
		usagePage, err = service.db.Coupons().ListUnapplied(ctx, 0, service.listingLimit, start)
		if err != nil {
			return Error.Wrap(err)
		}

		if err = service.applyCoupons(ctx, usagePage.Usages); err != nil {
			return Error.Wrap(err)
		}

		couponsUsages += len(usagePage.Usages)
	}

	service.log.Info("Number of processed coupons usages.", zap.Int("Coupons Usages", couponsUsages))
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
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("Stripe customer does not exist for coupon owner.", zap.Stringer("User ID", coupon.UserID), zap.Stringer("Coupon ID", coupon.ID))
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

// createInvoiceCouponItems consumes invoice project record and creates invoice line items for stripe customer.
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
		_, err = service.db.Coupons().Update(ctx, coupon.ID, payments.CouponUsed)
		if err != nil {
			return err
		}
	}

	projectItem := &stripe.InvoiceItemParams{
		Amount:      stripe.Int64(-usage.Amount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Customer:    stripe.String(customerID),
		Description: stripe.String(coupon.Description),
	}

	projectItem.AddMetadata("couponID", coupon.ID.String())

	_, err = service.stripeClient.InvoiceItems().New(projectItem)

	return err
}

// CreateInvoices lists through all customers and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 0, 0, 0, 0, 0, time.UTC)

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

	description := fmt.Sprintf("Tardigrade Cloud Storage for %s %d", period.Month(), period.Year())

	_, err = service.stripeClient.Invoices().New(
		&stripe.InvoiceParams{
			Customer:    stripe.String(cusID),
			AutoAdvance: stripe.Bool(service.AutoAdvance),
			Description: stripe.String(description),
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
		Storage: service.StorageMBMonthPriceCents.Mul(storageMBMonthDecimal(storage)).Round(0),
		Egress:  service.EgressMBPriceCents.Mul(egressMBDecimal(egress)).Round(0),
		Objects: service.ObjectMonthPriceCents.Mul(objectMonthDecimal(objects)).Round(0),
	}
}

// discountedProjectUsagePrice reduces the project usage price with the discount applied for the Stripe customer.
// The promotional coupons and bonus credits are not applied yet.
func (service *Service) discountedProjectUsagePrice(ctx context.Context, customerID string, projectUsagePrice int64) (int64, error) {
	customer, err := service.stripeClient.Customers().Get(customerID, nil)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	if customer.Discount == nil {
		return projectUsagePrice, nil
	}

	coupon := customer.Discount.Coupon

	if coupon == nil {
		return projectUsagePrice, nil
	}

	if !coupon.Valid {
		return projectUsagePrice, nil
	}

	if coupon.AmountOff > 0 {
		service.log.Info("Applying Stripe discount.", zap.String("Customer ID", customerID), zap.Int64("AmountOff", coupon.AmountOff))

		discounted := projectUsagePrice - coupon.AmountOff
		if discounted < 0 {
			return 0, nil
		}
		return discounted, nil
	}

	if coupon.PercentOff > 0 {
		service.log.Info("Applying Stripe discount.", zap.String("Customer ID", customerID), zap.Float64("PercentOff", coupon.PercentOff))

		discount := int64(math.Round(float64(projectUsagePrice) * coupon.PercentOff / 100))
		return projectUsagePrice - discount, nil
	}

	return projectUsagePrice, nil
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

// objectMonthDecimal converts objects usage from Object-Hours to Object-Months.
// The result is rounded to the nearest whole number, but returned as Decimal for convenience.
func objectMonthDecimal(objects float64) decimal.Decimal {
	return decimal.NewFromFloat(objects).Div(decimal.NewFromInt(hoursPerMonth)).Round(0)
}
