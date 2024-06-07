// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go/v75"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/currency"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
)

var (
	// Error defines stripecoinpayments service error.
	Error = errs.Class("stripecoinpayments service")

	mon = monkit.Package()

	_ healthcheck.HealthCheck = (*Service)(nil)
)

const (
	// hoursPerMonth is the number of months in a billing month. For the purpose of billing, the billing month is always 30 days.
	hoursPerMonth = 24 * 30

	storageInvoiceItemDesc = " - Storage (MB-Month)"
	egressInvoiceItemDesc  = " - Egress Bandwidth (MB)"
	segmentInvoiceItemDesc = " - Segment Fee (Segment-Month)"
)

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey        string `help:"stripe API secret key" default:""`
	StripePublicKey        string `help:"stripe API public key" default:""`
	StripeFreeTierCouponID string `help:"stripe free tier coupon ID" default:""`
	AutoAdvance            bool   `help:"toggle autoadvance feature for invoice creation" default:"false"`
	ListingLimit           int    `help:"sets the maximum amount of items before we start paging on requests" default:"100" hidden:"true"`
	SkipEmptyInvoices      bool   `help:"if set, skips the creation of empty invoices for customers with zero usage for the billing period" default:"true"`
	MaxParallelCalls       int    `help:"the maximum number of concurrent Stripe API calls in invoicing methods" default:"10"`
	RemoveExpiredCredit    bool   `help:"whether to remove expired package credit or not" default:"true"`
	UseIdempotency         bool   `help:"whether to use idempotency for create/update requests" default:"false"`
	EnableFreeTrialLogic   bool   `help:"whether to use users upgrade time and skip free tier status in billing process" default:"false"`
	Retries                RetryConfig
}

// Service is an implementation for payment service via Stripe and Coinpayments.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db        DB
	walletsDB storjscan.WalletsDB
	billingDB billing.TransactionsDB

	projectsDB   console.Projects
	usersDB      console.Users
	usageDB      accounting.ProjectAccounting
	stripeClient Client

	analytics *analytics.Service
	emission  *emission.Service

	usagePrices         payments.ProjectUsagePriceModel
	usagePriceOverrides map[string]payments.ProjectUsagePriceModel
	packagePlans        map[string]payments.PackagePlan
	partnerNames        []string
	// BonusRate amount of percents
	BonusRate int64
	// Coupon Values
	StripeFreeTierCouponID string

	// Stripe Extended Features
	AutoAdvance bool

	listingLimit         int
	skipEmptyInvoices    bool
	maxParallelCalls     int
	removeExpiredCredit  bool
	useIdempotency       bool
	enableFreeTrialLogic bool
	nowFn                func() time.Time
}

// NewService creates a Service instance.
func NewService(log *zap.Logger, stripeClient Client, config Config, db DB, walletsDB storjscan.WalletsDB, billingDB billing.TransactionsDB, projectsDB console.Projects, usersDB console.Users, usageDB accounting.ProjectAccounting, usagePrices payments.ProjectUsagePriceModel, usagePriceOverrides map[string]payments.ProjectUsagePriceModel, packagePlans map[string]payments.PackagePlan, bonusRate int64, analyticsService *analytics.Service, emissionService *emission.Service) (*Service, error) {
	var partners []string
	for partner := range usagePriceOverrides {
		partners = append(partners, partner)
	}

	return &Service{
		log:                    log,
		db:                     db,
		walletsDB:              walletsDB,
		billingDB:              billingDB,
		projectsDB:             projectsDB,
		usersDB:                usersDB,
		usageDB:                usageDB,
		stripeClient:           stripeClient,
		analytics:              analyticsService,
		emission:               emissionService,
		usagePrices:            usagePrices,
		usagePriceOverrides:    usagePriceOverrides,
		packagePlans:           packagePlans,
		partnerNames:           partners,
		BonusRate:              bonusRate,
		StripeFreeTierCouponID: config.StripeFreeTierCouponID,
		AutoAdvance:            config.AutoAdvance,
		listingLimit:           config.ListingLimit,
		skipEmptyInvoices:      config.SkipEmptyInvoices,
		maxParallelCalls:       config.MaxParallelCalls,
		removeExpiredCredit:    config.RemoveExpiredCredit,
		useIdempotency:         config.UseIdempotency,
		enableFreeTrialLogic:   config.EnableFreeTrialLogic,
		nowFn:                  time.Now,
	}, nil
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
}

// PrepareInvoiceProjectRecords iterates through all projects and creates invoice records if none exist.
func (service *Service) PrepareInvoiceProjectRecords(ctx context.Context, period time.Time, shouldAggregate bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var numberOfCustomers, numberOfRecords int
	customersPage := CustomersPage{
		Next: true,
	}

	for customersPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		customersPage, err = service.db.Customers().List(ctx, customersPage.Cursor, service.listingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}
		numberOfCustomers += len(customersPage.Customers)

		records, err := service.processCustomers(ctx, customersPage.Customers, shouldAggregate, start, end)
		if err != nil {
			return Error.Wrap(err)
		}
		numberOfRecords += records
	}

	service.log.Info("Number of processed entries.", zap.Int("Customers", numberOfCustomers), zap.Int("Projects", numberOfRecords))
	return nil
}

func (service *Service) processCustomers(ctx context.Context, customers []Customer, shouldAggregate bool, start, end time.Time) (int, error) {
	var regularRecords []CreateProjectRecord
	var recordsToAggregate []CreateProjectRecord
	for _, customer := range customers {
		if skip, err := service.mustSkipUser(ctx, customer.UserID); err != nil {
			return 0, Error.New("unable to determine if user must be skipped: %w", err)
		} else if skip {
			continue
		}

		projects, err := service.projectsDB.GetOwn(ctx, customer.UserID)
		if err != nil {
			return 0, Error.New("unable to get own projects: %w", err)
		}

		records, err := service.createProjectRecords(ctx, &customer, projects, start, end)
		if err != nil {
			return 0, Error.New("unable to create project records: %w", err)
		}

		// We generate 3 invoice items for each user project which means,
		// we can support only 83 projects in a single invoice (249 invoice items).
		if shouldAggregate && len(projects) > 83 {
			recordsToAggregate = append(recordsToAggregate, records...)
		} else {
			regularRecords = append(regularRecords, records...)
		}
	}

	err := service.db.ProjectRecords().Create(ctx, regularRecords, start, end)
	if err != nil {
		return 0, Error.New("failed to create regular project records: %w", err)
	}

	recordsCount := len(regularRecords)

	if shouldAggregate {
		err = service.db.ProjectRecords().CreateToBeAggregated(ctx, recordsToAggregate, start, end)
		if err != nil {
			return 0, Error.New("failed to create aggregated project records: %w", err)
		}

		recordsCount += len(recordsToAggregate)
	}

	return recordsCount, nil
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, customer *Customer, projects []console.Project, start, end time.Time) (_ []CreateProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if errors.Is(err, ErrProjectRecordExists) {
				service.log.Warn("Record for this project already exists.", zap.String("Customer ID", customer.ID), zap.String("Project ID", project.ID.String()))
				continue
			}

			return nil, err
		}

		from := start
		if service.enableFreeTrialLogic {
			from, err = service.getFromDate(ctx, customer.UserID, start, end)
			if err != nil {
				return nil, err
			}
		}

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, from, end)
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

	var totalRecords int
	var totalSkipped int

	for {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		// we are always starting from offset 0 because applyProjectRecords is changing project record state to applied
		recordsPage, err := service.db.ProjectRecords().ListUnapplied(ctx, uuid.UUID{}, service.listingLimit, start, end)
		if err != nil {
			return Error.Wrap(err)
		}
		totalRecords += len(recordsPage.Records)

		skipped, err := service.applyProjectRecords(ctx, recordsPage.Records, period)
		if err != nil {
			return Error.Wrap(err)
		}
		totalSkipped += skipped

		if !recordsPage.Next {
			break
		}
	}

	service.log.Info("Processed regular project records.",
		zap.Int("Total", totalRecords),
		zap.Int("Skipped", totalSkipped))
	return nil
}

// InvoiceApplyProjectRecordsGrouped iterates the customers and creates invoice items for each project and ensures line items are grouped by project.
func (service *Service) InvoiceApplyProjectRecordsGrouped(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var totalRecords atomic.Int64
	var totalSkipped atomic.Int64

	var mu sync.Mutex
	var errGrp errs.Group

	addErr := func(mu *sync.Mutex, err error) {
		mu.Lock()
		errGrp.Add(errs.Wrap(err))
		mu.Unlock()
	}

	limiter := sync2.NewLimiter(service.maxParallelCalls)
	defer func() {
		limiter.Wait()
	}()

	customersPage := CustomersPage{
		Next: true,
	}

	for customersPage.Next {
		customersPage, err = service.db.Customers().List(ctx, customersPage.Cursor, service.listingLimit, end)
		if err != nil {
			return err
		}
		for _, c := range customersPage.Customers {
			c := c
			limiter.Go(ctx, func() {
				skip, err := service.mustSkipUser(ctx, c.UserID)
				if err != nil {
					addErr(&mu, err)
					return
				}
				if skip {
					totalSkipped.Add(1)
					return
				}
				projects, err := service.projectsDB.GetOwn(ctx, c.UserID)
				if err != nil {
					addErr(&mu, err)
					return
				}

				projectIDs := []uuid.UUID{}
				projectNameMap := make(map[uuid.UUID]string)
				for _, p := range projects {
					projectIDs = append(projectIDs, p.ID)
					projectNameMap[p.ID] = p.Name
				}

				records, err := service.db.ProjectRecords().GetUnappliedByProjectIDs(ctx, projectIDs, start, end)
				if err != nil {
					addErr(&mu, err)
					return
				}

				for _, r := range records {
					totalRecords.Add(1)

					skipped, err := service.createInvoiceItems(ctx, c.BillingID, c.ID, projectNameMap[r.ProjectID], r, c.UserID, period)
					if err != nil {
						addErr(&mu, err)
						return
					}
					if skipped {
						totalSkipped.Add(1)
					}
				}
			})
		}
	}

	limiter.Wait()

	service.log.Info("Processed regular project records.",
		zap.Int64("Total", totalRecords.Load()),
		zap.Int64("Skipped", totalSkipped.Load()))
	return errGrp.Err()
}

// InvoiceApplyToBeAggregatedProjectRecords iterates through to be aggregated invoice project records and creates invoice line items
// for stripe customer.
func (service *Service) InvoiceApplyToBeAggregatedProjectRecords(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var totalRecords int
	var totalSkipped int

	for {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		// we are always starting from offset 0 because applyProjectRecords is changing project record state to applied
		recordsPage, err := service.db.ProjectRecords().ListToBeAggregated(ctx, uuid.UUID{}, service.listingLimit, start, end)
		if err != nil {
			return Error.Wrap(err)
		}
		totalRecords += len(recordsPage.Records)

		skipped, err := service.applyToBeAggregatedProjectRecords(ctx, recordsPage.Records, period)
		if err != nil {
			return Error.Wrap(err)
		}
		totalSkipped += skipped

		if !recordsPage.Next {
			break
		}
	}

	service.log.Info("Processed aggregated project records.",
		zap.Int("Total", totalRecords),
		zap.Int("Skipped", totalSkipped))
	return nil
}

// InvoiceApplyTokenBalance iterates through customer storjscan wallets and creates invoice credit notes
// for stripe customers with invoices on or after the given date.
func (service *Service) InvoiceApplyTokenBalance(ctx context.Context, createdOnAfter time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get all wallet entries
	wallets, err := service.walletsDB.GetAll(ctx)
	if err != nil {
		return Error.New("unable to get users in the wallets table")
	}

	var errGrp errs.Group

	for _, wallet := range wallets {
		// get the stripe customer invoice balance
		customerID, err := service.db.Customers().GetCustomerID(ctx, wallet.UserID)
		if err != nil {
			errGrp.Add(Error.New("unable to get stripe customer ID for user ID %s", wallet.UserID.String()))
			continue
		}
		customerInvoices, err := service.getInvoices(ctx, customerID, createdOnAfter)
		if err != nil {
			errGrp.Add(Error.New("unable to get invoice balance for stripe customer ID %s", customerID))
			continue
		}
		err = service.payInvoicesWithTokenBalance(ctx, customerID, wallet, customerInvoices)
		if err != nil {
			errGrp.Add(Error.New("unable to pay invoices for stripe customer ID %s", customerID))
			continue
		}
	}
	return errGrp.Err()
}

// InvoiceApplyCustomerTokenBalance creates invoice credit notes for the customers token payments to open invoices.
func (service *Service) InvoiceApplyCustomerTokenBalance(ctx context.Context, customerID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	userID, err := service.db.Customers().GetUserID(ctx, customerID)
	if err != nil {
		return Error.New("unable to get user ID for stripe customer ID %s", customerID)
	}

	customerInvoices, err := service.getInvoices(ctx, customerID, time.Unix(0, 0))
	if err != nil {
		return Error.New("error getting invoices for stripe customer %s", customerID)
	}

	return service.PayInvoicesWithTokenBalance(ctx, userID, customerID, customerInvoices)
}

// getInvoices returns the stripe customer's open finalized invoices created on or after the given date.
func (service *Service) getInvoices(ctx context.Context, cusID string, createdOnAfter time.Time) (_ []stripe.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   stripe.String(cusID),
		Status:     stripe.String(string(stripe.InvoiceStatusOpen)),
	}
	params.Filters.AddFilter("created", "gte", strconv.FormatInt(createdOnAfter.Unix(), 10))
	invoicesIterator := service.stripeClient.Invoices().List(params)
	var stripeInvoices []stripe.Invoice
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()
		if stripeInvoice != nil {
			stripeInvoices = append(stripeInvoices, *stripeInvoice)
		}
	}
	if err = invoicesIterator.Err(); err != nil {
		return stripeInvoices, Error.Wrap(err)
	}
	return stripeInvoices, nil
}

// addCreditNoteToInvoice creates a credit note for the user token payment.
func (service *Service) addCreditNoteToInvoice(ctx context.Context, invoiceID, cusID, wallet string, amount, txID int64) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	var lineParams []*stripe.CreditNoteLineParams

	lineParam := stripe.CreditNoteLineParams{
		Description: stripe.String("Storjscan Token payment"),
		Type:        stripe.String("custom_line_item"),
		UnitAmount:  stripe.Int64(amount),
		Quantity:    stripe.Int64(1),
	}

	lineParams = append(lineParams, &lineParam)

	params := &stripe.CreditNoteParams{
		Params:  stripe.Params{Context: ctx},
		Invoice: stripe.String(invoiceID),
		Lines:   lineParams,
		Memo:    stripe.String("Storjscan Token Payment - Wallet: " + wallet),
	}
	params.AddMetadata("txID", strconv.FormatInt(txID, 10))
	params.AddMetadata("wallet address", wallet)
	creditNote, err := service.stripeClient.CreditNotes().New(params)
	if err != nil {
		service.log.Warn("unable to add credit note for stripe customer", zap.String("Customer ID", cusID))
		return "", Error.Wrap(err)
	}
	return creditNote.ID, nil
}

// createTokenPaymentBillingTransaction creates a billing DB entry for the user token payment.
func (service *Service) createTokenPaymentBillingTransaction(ctx context.Context, userID uuid.UUID, invoiceID, wallet string, amount int64) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	metadata, err := json.Marshal(map[string]interface{}{
		"InvoiceID": invoiceID,
		"Wallet":    wallet,
	})

	transaction := billing.Transaction{
		UserID:      userID,
		Amount:      currency.AmountFromBaseUnits(amount, currency.USDollars),
		Description: "Paid Stripe Invoice",
		Source:      billing.StripeSource,
		Status:      billing.TransactionStatusPending,
		Type:        billing.TransactionTypeDebit,
		Metadata:    metadata,
		Timestamp:   time.Now(),
	}
	txIDs, err := service.billingDB.Insert(ctx, transaction)
	if err != nil {
		service.log.Warn("unable to add transaction to billing DB for user", zap.String("User ID", userID.String()))
		return 0, Error.Wrap(err)
	}
	return txIDs[0], nil
}

// applyProjectRecords applies invoice intents as invoice line items to stripe customer.
func (service *Service) applyProjectRecords(ctx context.Context, records []ProjectRecord, period time.Time) (skipCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	var mu sync.Mutex
	var errGrp errs.Group
	limiter := sync2.NewLimiter(service.maxParallelCalls)
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		limiter.Wait()
	}()

	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return 0, errs.Wrap(err)
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			// This should never happen, but be sure to log info to further troubleshoot before exiting.
			service.log.Error("project ID for corresponding project record not found", zap.Stringer("Record ID", record.ID), zap.Stringer("Project ID", record.ProjectID))
			return 0, errs.Wrap(err)
		}

		if skip, err := service.mustSkipUser(ctx, proj.OwnerID); err != nil {
			return 0, errs.Wrap(err)
		} else if skip {
			mu.Lock()
			skipCount++
			mu.Unlock()
			continue
		}

		billingID, cusID, err := service.db.Customers().GetStripeIDs(ctx, proj.OwnerID)
		if err != nil {
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("Stripe customer does not exist for project owner.", zap.Stringer("Owner ID", proj.OwnerID), zap.Stringer("Project ID", proj.ID))
				continue
			}

			return 0, errs.Wrap(err)
		}

		record := record
		limiter.Go(ctx, func() {
			skipped, err := service.createInvoiceItems(ctx, billingID, cusID, proj.Name, record, proj.OwnerID, period)
			if err != nil {
				mu.Lock()
				errGrp.Add(errs.Wrap(err))
				mu.Unlock()
				return
			}
			if skipped {
				mu.Lock()
				skipCount++
				mu.Unlock()
			}
		})
	}

	limiter.Wait()

	return skipCount, errGrp.Err()
}

// applyToBeAggregatedProjectRecords applies to be aggregated invoice intents as invoice line items to stripe customer.
func (service *Service) applyToBeAggregatedProjectRecords(ctx context.Context, records []ProjectRecord, period time.Time) (skipCount int, err error) {
	defer mon.Task()(&ctx)(&err)

	for _, record := range records {
		if err = ctx.Err(); err != nil {
			return 0, errs.Wrap(err)
		}

		proj, err := service.projectsDB.Get(ctx, record.ProjectID)
		if err != nil {
			service.log.Error("project ID for corresponding project record not found", zap.Stringer("Record ID", record.ID), zap.Stringer("Project ID", record.ProjectID))
			return 0, errs.Wrap(err)
		}

		if skip, err := service.mustSkipUser(ctx, proj.OwnerID); err != nil {
			return 0, errs.Wrap(err)
		} else if skip {
			skipCount++
			continue
		}

		cusID, err := service.db.Customers().GetCustomerID(ctx, proj.OwnerID)
		if err != nil {
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("Stripe customer does not exist for project owner.", zap.Stringer("Owner ID", proj.OwnerID), zap.Stringer("Project ID", proj.ID))
				continue
			}

			return 0, errs.Wrap(err)
		}

		record := record
		skipped, err := service.processProjectRecord(ctx, cusID, proj.Name, record, proj.OwnerID, period)
		if err != nil {
			return 0, errs.Wrap(err)
		}
		if skipped {
			skipCount++
		}
	}

	return skipCount, nil
}

// createInvoiceItems creates invoice line items for stripe customer.
func (service *Service) createInvoiceItems(ctx context.Context, billingID *string, cusID, projName string, record ProjectRecord, userID uuid.UUID, period time.Time) (skipped bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.useIdempotency {
		if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
			return false, err
		}
	}

	if service.skipEmptyInvoices && doesProjectRecordHaveNoUsage(record) {
		if service.useIdempotency {
			if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
				return false, err
			}
		}

		return true, nil
	}

	from := record.PeriodStart
	if service.enableFreeTrialLogic {
		from, err = service.getFromDate(ctx, userID, record.PeriodStart, record.PeriodEnd)
		if err != nil {
			return false, err
		}
	}

	usages, err := service.usageDB.GetProjectTotalByPartner(ctx, record.ProjectID, service.partnerNames, from, record.PeriodEnd)
	if err != nil {
		return false, err
	}

	items := service.InvoiceItemsFromProjectUsage(projName, usages, false)

	var invoiceID *string
	if billingID == nil {
		billingID = &cusID
	} else {
		// create parent invoice
		invoiceID, err = service.createParentInvoice(ctx, *billingID, cusID, projName, period)
	}
	for _, item := range items {
		item.Params = stripe.Params{Context: ctx}
		item.Currency = stripe.String(string(stripe.CurrencyUSD))
		item.Customer = stripe.String(*billingID)
		if invoiceID != nil {
			item.Invoice = invoiceID
		}
		// TODO: do not expose regular project ID.
		item.AddMetadata("projectID", record.ProjectID.String())

		if service.useIdempotency {
			item.SetIdempotencyKey(getIdempotencyKey(record.ProjectID, *item.Description, period))
		}

		_, err = service.stripeClient.InvoiceItems().New(item)
		if err != nil {
			return false, err
		}
	}

	if service.useIdempotency {
		if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
			return false, err
		}
	}

	return false, nil
}

type usage int

const (
	storage usage = 0
	egress  usage = 1
	segment usage = 2
)

// processProjectRecord creates or updates invoice line items for stripe customer.
func (service *Service) processProjectRecord(ctx context.Context, cusID, projName string, record ProjectRecord, userID uuid.UUID, period time.Time) (skipped bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.useIdempotency {
		if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
			return false, err
		}
	}

	if service.skipEmptyInvoices && doesProjectRecordHaveNoUsage(record) {
		if service.useIdempotency {
			if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
				return false, err
			}
		}

		return true, nil
	}

	from := record.PeriodStart
	if service.enableFreeTrialLogic {
		from, err = service.getFromDate(ctx, userID, record.PeriodStart, record.PeriodEnd)
		if err != nil {
			return false, err
		}
	}

	usages, err := service.usageDB.GetProjectTotalByPartner(ctx, record.ProjectID, service.partnerNames, from, record.PeriodEnd)
	if err != nil {
		return false, err
	}

	newItems := service.InvoiceItemsFromProjectUsage(projName, usages, true)

	existingItems, err := service.getExistingInvoiceItems(ctx, cusID)
	if err != nil {
		return false, err
	}

	if existingItems[segment] == nil || existingItems[storage] == nil || existingItems[egress] == nil {
		for _, item := range newItems {
			item.Params = stripe.Params{Context: ctx}
			item.Currency = stripe.String(string(stripe.CurrencyUSD))
			item.Customer = stripe.String(cusID)
			// TODO: do not expose regular project ID.
			item.AddMetadata("projectID", record.ProjectID.String())

			if service.useIdempotency {
				item.SetIdempotencyKey(getIdempotencyKey(record.ProjectID, *item.Description, period))
			}

			_, err = service.stripeClient.InvoiceItems().New(item)
			if err != nil {
				return false, err
			}
		}
	} else {
		err = service.updateExistingInvoiceItems(ctx, existingItems, newItems, record.ProjectID, period)
		if err != nil {
			return false, err
		}
	}

	if service.useIdempotency {
		if err = service.db.ProjectRecords().Consume(ctx, record.ID); err != nil {
			return false, err
		}
	}

	return false, nil
}

// getIdempotencyKey creates new unique idempotency key for given invoice item.
func getIdempotencyKey(projectID uuid.UUID, itemDesc string, period time.Time) string {
	// We can't just use item.Description because it includes project name.
	// There is a chance project name can be updated by the user during invoicing process.
	itemIdentifier := itemDesc
	if strings.Contains(itemDesc, storageInvoiceItemDesc) {
		itemIdentifier = "storage"
	} else if strings.Contains(itemDesc, egressInvoiceItemDesc) {
		itemIdentifier = "egress"
	} else if strings.Contains(itemDesc, segmentInvoiceItemDesc) {
		itemIdentifier = "segment"
	}

	key := fmt.Sprintf("%s-%s-%s", projectID, itemIdentifier, period.Format("2006-01"))
	key = strings.ToLower(strings.ReplaceAll(key, " ", "-"))

	return key
}

// getExistingInvoiceItems lists 3 existing pending invoice line items for stripe customer.
func (service *Service) getExistingInvoiceItems(ctx context.Context, cusID string) (map[usage]*stripe.InvoiceItem, error) {
	existingItemsIter := service.stripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
		Customer: &cusID,
		Pending:  stripe.Bool(true),
		ListParams: stripe.ListParams{
			Context: ctx,
			Limit:   stripe.Int64(3),
		},
	})

	items := map[usage]*stripe.InvoiceItem{
		storage: nil,
		egress:  nil,
		segment: nil,
	}

	for existingItemsIter.Next() {
		item := existingItemsIter.InvoiceItem()
		if strings.Contains(item.Description, storageInvoiceItemDesc) {
			items[storage] = item
		} else if strings.Contains(item.Description, egressInvoiceItemDesc) {
			items[egress] = item
		} else if strings.Contains(item.Description, segmentInvoiceItemDesc) {
			items[segment] = item
		}
	}

	return items, existingItemsIter.Err()
}

// updateExistingInvoiceItems updates 3 existing pending invoice line items for stripe customer.
func (service *Service) updateExistingInvoiceItems(ctx context.Context, existingItems map[usage]*stripe.InvoiceItem, newItems []*stripe.InvoiceItemParams, projectID uuid.UUID, period time.Time) (err error) {
	for _, item := range newItems {
		if strings.Contains(*item.Description, storageInvoiceItemDesc) {
			existingItems[storage].Quantity += *item.Quantity
		} else if strings.Contains(*item.Description, egressInvoiceItemDesc) {
			existingItems[egress].Quantity += *item.Quantity
		} else if strings.Contains(*item.Description, segmentInvoiceItemDesc) {
			existingItems[segment].Quantity += *item.Quantity
		}
	}

	for _, item := range existingItems {
		params := &stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Quantity: stripe.Int64(item.Quantity),
		}

		if service.useIdempotency {
			params.SetIdempotencyKey(getIdempotencyKey(projectID, item.Description, period))
		}

		_, err = service.stripeClient.InvoiceItems().Update(item.ID, params)
		if err != nil {
			return err
		}
	}

	return nil
}

// InvoiceItemsFromProjectUsage calculates Stripe invoice item from project usage.
func (service *Service) InvoiceItemsFromProjectUsage(projName string, partnerUsages map[string]accounting.ProjectUsage, aggregated bool) (result []*stripe.InvoiceItemParams) {
	var partners []string
	if len(partnerUsages) == 0 {
		partners = []string{""}
		partnerUsages = map[string]accounting.ProjectUsage{"": {}}
	} else {
		for partner := range partnerUsages {
			partners = append(partners, partner)
		}
		sort.Strings(partners)
	}

	for _, partner := range partners {
		priceModel := service.Accounts().GetProjectUsagePriceModel(partner)

		usage := partnerUsages[partner]
		usage.Egress = applyEgressDiscount(usage, priceModel)

		prefix := "Project " + projName
		if partner != "" {
			prefix += " (" + partner + ")"
		}

		if aggregated {
			prefix = "All projects"
		}

		projectItem := &stripe.InvoiceItemParams{}
		projectItem.Description = stripe.String(prefix + storageInvoiceItemDesc)
		projectItem.Quantity = stripe.Int64(storageMBMonthDecimal(usage.Storage).IntPart())
		storagePrice, _ := priceModel.StorageMBMonthCents.Float64()
		projectItem.UnitAmountDecimal = stripe.Float64(storagePrice)
		result = append(result, projectItem)

		projectItem = &stripe.InvoiceItemParams{}
		projectItem.Description = stripe.String(prefix + egressInvoiceItemDesc)
		projectItem.Quantity = stripe.Int64(egressMBDecimal(usage.Egress).IntPart())
		egressPrice, _ := priceModel.EgressMBCents.Float64()
		projectItem.UnitAmountDecimal = stripe.Float64(egressPrice)
		result = append(result, projectItem)

		projectItem = &stripe.InvoiceItemParams{}
		projectItem.Description = stripe.String(prefix + segmentInvoiceItemDesc)
		projectItem.Quantity = stripe.Int64(segmentMonthDecimal(usage.SegmentCount).IntPart())
		segmentPrice, _ := priceModel.SegmentMonthCents.Float64()
		projectItem.UnitAmountDecimal = stripe.Float64(segmentPrice)
		result = append(result, projectItem)
	}

	service.log.Info("invoice items", zap.Any("result", result))

	return result
}

// RemoveExpiredPackageCredit removes a user's package plan credit, or sends an analytics event, if it has expired.
// If the user has never received credit from anything other than the package, and it is expired, the remaining package
// credit is removed. If the user has received credit from another source, we send an analytics event instead of removing
// the remaining credit so someone can remove it manually. `sentEvent` indicates whether this analytics event was sent.
func (service *Service) RemoveExpiredPackageCredit(ctx context.Context, customer Customer) (sentEvent bool, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: store the package expiration somewhere
	if customer.PackagePlan == nil || customer.PackagePurchasedAt == nil ||
		customer.PackagePurchasedAt.After(service.nowFn().AddDate(-1, -1, 0)) {
		return false, nil
	}
	list := service.stripeClient.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{
		Customer: stripe.String(customer.ID),
	})

	var balance int64
	var gotBalance, foundOtherCredit bool
	var tx *stripe.CustomerBalanceTransaction

	for list.Next() {
		tx = list.CustomerBalanceTransaction()
		if !gotBalance {
			// Stripe returns list ordered by most recent, so ending balance of the first item is current balance.
			balance = tx.EndingBalance
			gotBalance = true
			// if user doesn't have credit, we're done.
			if balance >= 0 {
				break
			}
		}

		// negative amount means credit
		if tx.Amount < 0 {
			if tx.Description != *customer.PackagePlan {
				foundOtherCredit = true
			}
		}
	}

	// send analytics event to notify someone to handle removing credit if credit other than package exists.
	if foundOtherCredit {
		if service.analytics != nil {
			service.analytics.TrackExpiredCreditNeedsRemoval(customer.UserID, customer.ID, *customer.PackagePlan)
		}
		return true, nil
	}

	// If no other credit found, we can set the balance to zero.
	if balance < 0 {
		_, err = service.stripeClient.CustomerBalanceTransactions().New(&stripe.CustomerBalanceTransactionParams{
			Customer:    stripe.String(customer.ID),
			Amount:      stripe.Int64(-balance),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Description: stripe.String(fmt.Sprintf("%s expired", *customer.PackagePlan)),
		})
		if err != nil {
			return false, Error.Wrap(err)
		}
		if service.analytics != nil {
			service.analytics.TrackExpiredCreditRemoved(customer.UserID, customer.ID, *customer.PackagePlan)
		}
	}

	err = service.Accounts().UpdatePackage(ctx, customer.UserID, nil, nil)

	return false, Error.Wrap(err)
}

// ApplyFreeTierCoupons iterates through all customers in Stripe. For each customer,
// if that customer does not currently have a Stripe coupon, the free tier Stripe coupon
// is applied.
func (service *Service) ApplyFreeTierCoupons(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	customers := service.db.Customers()

	limiter := sync2.NewLimiter(service.maxParallelCalls)
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		limiter.Wait()
	}()

	var mu sync.Mutex
	var appliedCoupons int
	failedUsers := []string{}
	morePages := true
	var nextCursor uuid.UUID
	listingLimit := 100
	end := time.Now()
	for morePages {
		customersPage, err := customers.List(ctx, nextCursor, listingLimit, end)
		if err != nil {
			return err
		}
		morePages = customersPage.Next
		nextCursor = customersPage.Cursor

		for _, c := range customersPage.Customers {
			c := c
			limiter.Go(ctx, func() {
				if skip, err := service.mustSkipUser(ctx, c.UserID); err != nil {
					mu.Lock()
					failedUsers = append(failedUsers, c.ID)
					mu.Unlock()
					return
				} else if skip {
					return
				}

				applied, err := service.applyFreeTierCoupon(ctx, c.ID)
				if err != nil {
					mu.Lock()
					failedUsers = append(failedUsers, c.ID)
					mu.Unlock()
					return
				}
				if applied {
					mu.Lock()
					appliedCoupons++
					mu.Unlock()
				}
			})
		}
	}

	limiter.Wait()

	if len(failedUsers) > 0 {
		service.log.Warn("Failed to get or apply free tier coupon to some customers:", zap.String("idlist", strings.Join(failedUsers, ", ")))
	}
	service.log.Info("Finished", zap.Int("number of coupons applied", appliedCoupons))

	return nil
}

// applyFreeTierCoupon applies the free tier Stripe coupon to a customer if it doesn't already have a coupon.
func (service *Service) applyFreeTierCoupon(ctx context.Context, cusID string) (applied bool, err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}}
	stripeCust, err := service.stripeClient.Customers().Get(cusID, params)
	if err != nil {
		service.log.Error("Failed to get customer", zap.Error(err))
		return false, err
	}

	// if customer has a coupon, don't apply the free tier coupon
	if stripeCust.Discount != nil && stripeCust.Discount.Coupon != nil {
		return false, nil
	}

	params = &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
		Coupon: stripe.String(service.StripeFreeTierCouponID),
	}
	_, err = service.stripeClient.Customers().Update(cusID, params)
	if err != nil {
		service.log.Error("Failed to update customer with free tier coupon", zap.Error(err))
		return false, err
	}

	return true, nil
}

// CreateInvoices lists through all customers, removes expired credit if applicable, and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context, period time.Time, includeEmissionInfo bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var nextCursor uuid.UUID
	var totalDraft, totalScheduled int
	for {
		cusPage, err := service.db.Customers().List(ctx, nextCursor, service.listingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if service.removeExpiredCredit {
			for _, c := range cusPage.Customers {
				if c.PackagePlan != nil {
					if _, err := service.RemoveExpiredPackageCredit(ctx, c); err != nil {
						return Error.Wrap(err)
					}
				}
			}
		}

		scheduled, draft, err := service.createInvoices(ctx, cusPage.Customers, start, includeEmissionInfo)
		if err != nil {
			return Error.Wrap(err)
		}
		totalScheduled += scheduled
		totalDraft += draft

		if !cusPage.Next {
			break
		}
		nextCursor = cusPage.Cursor
	}

	service.log.Info("Number of created invoices", zap.Int("Draft", totalDraft), zap.Int("Scheduled", totalScheduled))
	return nil
}

// createInvoice creates invoice for Stripe customer.
func (service *Service) createInvoice(ctx context.Context, cusID string, period time.Time, includeEmissionInfo bool) (stripeInvoice *stripe.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	var footer *string

	if includeEmissionInfo {
		var (
			lastItemID   string
			totalStorage int64
			hasItems     bool
		)

		for {
			params := &stripe.InvoiceItemListParams{
				Customer: &cusID,
				Pending:  stripe.Bool(true),
				ListParams: stripe.ListParams{
					Context: ctx,
					Limit:   stripe.Int64(100), // Max limit per request
				},
			}
			if lastItemID != "" {
				params.ListParams.StartingAfter = stripe.String(lastItemID)
			}

			itemsIter := service.stripeClient.InvoiceItems().List(params)
			for itemsIter.Next() {
				if !hasItems {
					hasItems = true
				}

				item := itemsIter.InvoiceItem()
				if strings.Contains(item.Description, storageInvoiceItemDesc) {
					totalStorage += item.Quantity
				}

				lastItemID = item.ID
			}

			if err = itemsIter.Err(); err != nil {
				return nil, err
			}
			if !hasItems {
				return nil, nil
			}

			// Use HasMore to determine if we should break the loop.
			if !itemsIter.InvoiceItemList().HasMore {
				break
			}
		}

		impact, err := service.emission.CalculateImpact(&emission.CalculationInput{
			AmountOfDataInTB: float64(totalStorage * hoursPerMonth / 1000000), // convert MB-month to TB-hour.
			Duration:         time.Hour * hoursPerMonth,
			IsTBDuration:     true,
		})
		if err != nil {
			return nil, err
		}

		whitePaperLink := "https://www.storj.io/documents/storj-sustainability-whitepaper.pdf"
		footerMsg := fmt.Sprintf(
			"Estimated Storj Emissions: %.3f kgCO2e\nEstimated Hyperscaler Emissions: %.3f kgCO2e\nMore information on estimates: %s",
			impact.EstimatedKgCO2eStorj,
			impact.EstimatedKgCO2eHyperscaler,
			whitePaperLink,
		)

		savedValue := impact.EstimatedKgCO2eHyperscaler - impact.EstimatedKgCO2eStorj
		if savedValue < 0 {
			savedValue = 0
		}

		savedTrees := service.emission.CalculateSavedTrees(savedValue)
		if savedTrees > 0 {
			treesCalcLink := "https://www.epa.gov/energy/greenhouse-gases-equivalencies-calculator-calculations-and-references#seedlings"
			footerMsg += fmt.Sprintf("\nEstimated Trees Saved: %d\nMore information on trees saved: %s", savedTrees, treesCalcLink)
		}

		footerMsg += "\n\nNote: The carbon emissions displayed are estimated based on the total account usage, calculated for the dates of this invoice."

		footer = stripe.String(footerMsg)
	} else {
		itemsIter := service.stripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
			Customer: &cusID,
			Pending:  stripe.Bool(true),
			ListParams: stripe.ListParams{
				Context: ctx,
				Limit:   stripe.Int64(1),
			},
		})

		hasItems := itemsIter.Next()
		if err = itemsIter.Err(); err != nil {
			return nil, err
		}
		if !hasItems {
			return nil, nil
		}
	}

	description := fmt.Sprintf("Storj Cloud Storage for %s %d", period.Month(), period.Year())
	stripeInvoice, err = service.stripeClient.Invoices().New(
		&stripe.InvoiceParams{
			Params:                      stripe.Params{Context: ctx},
			Customer:                    stripe.String(cusID),
			AutoAdvance:                 stripe.Bool(service.AutoAdvance),
			Description:                 stripe.String(description),
			PendingInvoiceItemsBehavior: stripe.String("include"),
			Footer:                      footer,
		},
	)
	if err != nil {
		return nil, err
	}

	// auto advance the invoice if nothing is due from the customer
	if !stripeInvoice.AutoAdvance && stripeInvoice.AmountDue == 0 {
		params := &stripe.InvoiceParams{
			Params:      stripe.Params{Context: ctx},
			AutoAdvance: stripe.Bool(true),
		}
		stripeInvoice, err = service.stripeClient.Invoices().Update(stripeInvoice.ID, params)
		if err != nil {
			return nil, err
		}
	}

	return stripeInvoice, nil
}

// createInvoices creates invoices for Stripe customers.
func (service *Service) createInvoices(ctx context.Context, customers []Customer, period time.Time, includeEmissionInfo bool) (scheduled, draft int, err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(service.maxParallelCalls)
	var errGrp errs.Group
	var mu sync.Mutex

	for _, cus := range customers {
		cus := cus
		limiter.Go(ctx, func() {
			if skip, err := service.mustSkipUser(ctx, cus.UserID); err != nil {
				mu.Lock()
				errGrp.Add(err)
				mu.Unlock()
				return
			} else if skip {
				return
			}

			inv, err := service.createInvoice(ctx, cus.ID, period, includeEmissionInfo)
			if err != nil {
				mu.Lock()
				errGrp.Add(err)
				mu.Unlock()
				return
			}
			if inv != nil {
				mu.Lock()
				if inv.AutoAdvance {
					scheduled++
				} else {
					draft++
				}
				mu.Unlock()
			}
		})
	}

	limiter.Wait()

	return scheduled, draft, errGrp.Err()
}

// createParentInvoice creates a parent invoice for the customer.
func (service *Service) createParentInvoice(ctx context.Context, billingID, cusID, projName string, period time.Time) (invoiceID *string, err error) {
	defer mon.Task()(&ctx)(&err)

	description := fmt.Sprintf("Storj Cloud Storage for child project %s and period %s %d", projName, period.UTC().Month(), period.UTC().Year())
	stripeInvoice, err := service.stripeClient.Invoices().New(
		&stripe.InvoiceParams{
			Params:                      stripe.Params{Context: ctx},
			Customer:                    stripe.String(billingID),
			AutoAdvance:                 stripe.Bool(false),
			Description:                 stripe.String(description),
			PendingInvoiceItemsBehavior: stripe.String("exclude"),
			Metadata:                    map[string]string{"Child Account": cusID},
		},
	)
	if err != nil {
		return nil, err
	}
	return &stripeInvoice.ID, nil
}

// SetInvoiceStatus will set all open invoices within the specified date range to the requested status.
func (service *Service) SetInvoiceStatus(ctx context.Context, startPeriod, endPeriod time.Time, status string, dryRun bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	switch stripe.InvoiceStatus(strings.ToLower(status)) {
	case stripe.InvoiceStatusUncollectible:
		err = service.iterateInvoicesInTimeRange(ctx, startPeriod, endPeriod, func(invoiceId string) error {
			service.log.Info("updating invoice status to uncollectible", zap.String("invoiceId", invoiceId))
			if !dryRun {
				_, err := service.stripeClient.Invoices().MarkUncollectible(invoiceId, &stripe.InvoiceMarkUncollectibleParams{})
				if err != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		})
	case stripe.InvoiceStatusVoid:
		err = service.iterateInvoicesInTimeRange(ctx, startPeriod, endPeriod, func(invoiceId string) error {
			service.log.Info("updating invoice status to void", zap.String("invoiceId", invoiceId))
			if !dryRun {
				_, err = service.stripeClient.Invoices().VoidInvoice(invoiceId, &stripe.InvoiceVoidInvoiceParams{})
				if err != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		})
	case stripe.InvoiceStatusPaid:
		err = service.iterateInvoicesInTimeRange(ctx, startPeriod, endPeriod, func(invoiceId string) error {
			service.log.Info("updating invoice status to paid", zap.String("invoiceId", invoiceId))
			if !dryRun {
				payParams := &stripe.InvoicePayParams{
					Params:        stripe.Params{Context: ctx},
					PaidOutOfBand: stripe.Bool(true),
				}
				_, err = service.stripeClient.Invoices().Pay(invoiceId, payParams)
				if err != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		})
	default:
		// unknown
		service.log.Error("Unknown status provided. Valid options are uncollectible, void, or paid.", zap.String("status", status))
		return Error.New("unknown status provided")
	}
	return err
}

func (service *Service) iterateInvoicesInTimeRange(ctx context.Context, startPeriod, endPeriod time.Time, updateStatus func(string) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{
			Context: ctx,
			Limit:   stripe.Int64(100),
		},
		Status: stripe.String("open"),
		CreatedRange: &stripe.RangeQueryParams{
			GreaterThanOrEqual: startPeriod.Unix(),
			LesserThanOrEqual:  endPeriod.Unix(),
		},
	}

	numInvoices := 0
	invoicesIterator := service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		numInvoices++
		stripeInvoice := invoicesIterator.Invoice()

		err := updateStatus(stripeInvoice.ID)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	service.log.Info("found " + strconv.Itoa(numInvoices) + " total invoices")
	return Error.Wrap(invoicesIterator.Err())
}

// CreateBalanceInvoiceItems will find users with a stripe balance, create an invoice
// item with the charges due, and zero out the stripe balance.
func (service *Service) CreateBalanceInvoiceItems(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	custListParams := &stripe.CustomerListParams{
		ListParams: stripe.ListParams{
			Context: ctx,
			Limit:   stripe.Int64(100),
		},
	}

	var errGrp errs.Group
	itr := service.stripeClient.Customers().List(custListParams)
	for itr.Next() {
		if itr.Customer().Balance <= 0 {
			continue
		}

		userID, err := service.db.Customers().GetUserID(ctx, itr.Customer().ID)
		if err != nil {
			return err
		}
		if skip, err := service.mustSkipUser(ctx, userID); err != nil {
			return err
		} else if skip {
			continue
		}

		service.log.Info("Creating invoice item for customer prior balance", zap.String("CustomerID", itr.Customer().ID))
		itemParams := &stripe.InvoiceItemParams{
			Params: stripe.Params{
				Context: ctx,
			},
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Customer:    stripe.String(itr.Customer().ID),
			Description: stripe.String("Prior Stripe Customer Balance"),
			Quantity:    stripe.Int64(1),
			UnitAmount:  stripe.Int64(itr.Customer().Balance),
		}
		invoiceItem, err := service.stripeClient.InvoiceItems().New(itemParams)
		if err != nil {
			service.log.Error("Failed to add invoice item for customer prior balance", zap.Error(err))
			errGrp.Add(err)
			continue
		}
		service.log.Info("Updating customer balance to 0", zap.String("CustomerID", itr.Customer().ID))
		balanceParams := &stripe.CustomerBalanceTransactionParams{
			Params: stripe.Params{
				Context: ctx,
			},
			Amount:      stripe.Int64(-itr.Customer().Balance),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Customer:    stripe.String(itr.Customer().ID),
			Description: stripe.String("Customer balance adjusted to 0 after adding invoice item " + invoiceItem.ID),
		}
		_, err = service.stripeClient.CustomerBalanceTransactions().New(balanceParams)
		if err != nil {
			service.log.Error("Failed to update customer balance to 0 after adding invoice item", zap.Error(err))
			errGrp.Add(err)
			continue
		}
		service.log.Info("Customer successfully updated", zap.String("CustomerID", itr.Customer().ID), zap.Int64("Prior Balance", itr.Customer().Balance), zap.Int64("New Balance", 0), zap.String("InvoiceItemID", invoiceItem.ID))
	}
	if itr.Err() != nil {
		service.log.Error("Failed to create invoice items for all customers", zap.Error(itr.Err()))
		errGrp.Add(itr.Err())
	}
	return errGrp.Err()
}

// GenerateInvoices performs tasks necessary to generate Stripe invoices.
// This is equivalent to invoking PrepareInvoiceProjectRecords, InvoiceApplyProjectRecords,
// and CreateInvoices in order.
func (service *Service) GenerateInvoices(ctx context.Context, period time.Time, shouldAggregate, groupInvoiceItems, includeEmissionInfo bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("Preparing invoice project records")
	err = service.PrepareInvoiceProjectRecords(ctx, period, shouldAggregate)
	if err != nil {
		return err
	}

	service.log.Info("Applying invoice project records")
	if groupInvoiceItems {
		err = service.InvoiceApplyProjectRecordsGrouped(ctx, period)
		if err != nil {
			return err
		}
	} else {
		err = service.InvoiceApplyProjectRecords(ctx, period)
		if err != nil {
			return err
		}
	}

	if shouldAggregate {
		service.log.Info("Applying to be aggregated invoice project records")
		err = service.InvoiceApplyToBeAggregatedProjectRecords(ctx, period)
		if err != nil {
			return err
		}
	}

	service.log.Info("Creating invoices")
	err = service.CreateInvoices(ctx, period, includeEmissionInfo)
	if err != nil {
		return err
	}

	return nil
}

// FinalizeInvoices transitions all draft invoices to open finalized invoices in stripe. No payment is to be collected yet.
func (service *Service) FinalizeInvoices(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Status:     stripe.String("draft"),
	}

	invoicesIterator := service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		userID, err := service.db.Customers().GetUserID(ctx, stripeInvoice.Customer.ID)
		if err != nil {
			if errors.Is(err, ErrNoCustomer) {
				service.log.Warn("User ID does not exist for invoiced customer.", zap.String("stripe customer", stripeInvoice.Customer.ID))
				continue
			}
			return Error.Wrap(err)
		}
		if skip, err := service.mustSkipUser(ctx, userID); err != nil {
			return Error.Wrap(err)
		} else if skip {
			continue
		}

		if stripeInvoice.AutoAdvance {
			continue
		}

		err = service.finalizeInvoice(ctx, stripeInvoice.ID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return Error.Wrap(invoicesIterator.Err())
}

func (service *Service) finalizeInvoice(ctx context.Context, invoiceID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceFinalizeInvoiceParams{
		Params:      stripe.Params{Context: ctx},
		AutoAdvance: stripe.Bool(false),
	}
	_, err = service.stripeClient.Invoices().FinalizeInvoice(invoiceID, params)
	return err
}

// PayInvoices attempts to transition all open finalized invoices created on or after a certain time to "paid"
// by charging the customer according to subscriptions settings.
func (service *Service) PayInvoices(ctx context.Context, createdOnAfter time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Status:     stripe.String("open"),
	}
	params.Filters.AddFilter("created", "gte", strconv.FormatInt(createdOnAfter.Unix(), 10))

	invoicesIterator := service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()
		if stripeInvoice.DueDate > 0 {
			service.log.Info("Skipping invoice marked for manual payment",
				zap.String("id", stripeInvoice.ID),
				zap.String("number", stripeInvoice.Number),
				zap.String("customer", stripeInvoice.Customer.ID))
			continue
		}

		params := &stripe.InvoicePayParams{Params: stripe.Params{Context: ctx}}
		_, err = service.stripeClient.Invoices().Pay(stripeInvoice.ID, params)
		if err != nil {
			service.log.Warn("unable to pay invoice",
				zap.String("stripe-invoice-id", stripeInvoice.ID),
				zap.Error(err))
			continue
		}
	}
	return invoicesIterator.Err()
}

// PayCustomerInvoices attempts to transition all open finalized invoices created on or after a certain time to "paid"
// by charging the customer according to subscriptions settings.
func (service *Service) PayCustomerInvoices(ctx context.Context, customerID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	userID, err := service.db.Customers().GetUserID(ctx, customerID)
	if err != nil {
		return Error.Wrap(err)
	}
	if skip, err := service.mustSkipUser(ctx, userID); err != nil {
		return Error.Wrap(err)
	} else if skip {
		return Error.New("customer %s is inactive", customerID)
	}

	customerInvoices, err := service.getInvoices(ctx, customerID, time.Unix(0, 0))
	if err != nil {
		return Error.New("error getting invoices for stripe customer %s", customerID)
	}

	var errGrp errs.Group
	for _, customerInvoice := range customerInvoices {
		if customerInvoice.DueDate > 0 {
			service.log.Info("Skipping invoice marked for manual payment",
				zap.String("id", customerInvoice.ID),
				zap.String("number", customerInvoice.Number),
				zap.String("customer", customerInvoice.Customer.ID))
			continue
		}

		params := &stripe.InvoicePayParams{Params: stripe.Params{Context: ctx}}
		_, err = service.stripeClient.Invoices().Pay(customerInvoice.ID, params)
		if err != nil {
			errGrp.Add(Error.New("unable to pay invoice %s", customerInvoice.ID))
			continue
		}
	}
	return errGrp.Err()
}

// PayInvoicesWithTokenBalance attempts to transition all the users open invoices to "paid" by charging the customer
// token balance.
func (service *Service) PayInvoicesWithTokenBalance(ctx context.Context, userID uuid.UUID, cusID string, invoices []stripe.Invoice) (err error) {
	// get wallet
	wallet, err := service.walletsDB.GetWallet(ctx, userID)
	if err != nil {
		return Error.New("unable to get users in the wallets table")
	}

	return service.payInvoicesWithTokenBalance(ctx, cusID, storjscan.Wallet{
		UserID:  userID,
		Address: wallet,
	}, invoices)
}

// FailPendingInvoiceTokenPayments marks all specified pending invoice token payments as failed, and refunds the pending charges.
func (service *Service) FailPendingInvoiceTokenPayments(ctx context.Context, pendingPayments []string) (err error) {
	defer mon.Task()(&ctx)(&err)

	txIDs := make([]int64, len(pendingPayments))

	for i, s := range pendingPayments {
		txIDs[i], _ = strconv.ParseInt(s, 10, 64)
	}

	return service.billingDB.FailPendingInvoiceTokenPayments(ctx, txIDs...)
}

// CompletePendingInvoiceTokenPayments updates the status of the pending invoice token payment to complete.
func (service *Service) CompletePendingInvoiceTokenPayments(ctx context.Context, pendingPayments []string) (err error) {
	defer mon.Task()(&ctx)(&err)

	txIDs := make([]int64, len(pendingPayments))

	for i, s := range pendingPayments {
		txIDs[i], _ = strconv.ParseInt(s, 10, 64)
	}

	return service.billingDB.CompletePendingInvoiceTokenPayments(ctx, txIDs...)
}

// payInvoicesWithTokenBalance attempts to transition the users open invoices to "paid" by charging the customer
// token balance.
func (service *Service) payInvoicesWithTokenBalance(ctx context.Context, cusID string, wallet storjscan.Wallet, invoices []stripe.Invoice) (err error) {
	defer mon.Task()(&ctx)(&err)

	var errGrp errs.Group

	for _, invoice := range invoices {
		// if no balance due, do nothing
		if invoice.AmountRemaining <= 0 {
			continue
		}
		monetaryTokenBalance, err := service.billingDB.GetBalance(ctx, wallet.UserID)
		if err != nil {
			errGrp.Add(Error.New("unable to get balance for user ID %s", wallet.UserID.String()))
			continue
		}
		// truncate here since stripe only has cent level precision for invoices.
		// The users account balance will still maintain the full precision monetary value!
		tokenBalance := currency.AmountFromDecimal(monetaryTokenBalance.AsDecimal().Truncate(2), currency.USDollars)
		// if token balance is not > 0, don't bother with the rest
		if tokenBalance.BaseUnits() <= 0 {
			break
		}

		var tokenCreditAmount int64
		if invoice.AmountRemaining >= tokenBalance.BaseUnits() {
			tokenCreditAmount = tokenBalance.BaseUnits()
		} else {
			tokenCreditAmount = invoice.AmountRemaining
		}

		txID, err := service.createTokenPaymentBillingTransaction(ctx, wallet.UserID, invoice.ID, wallet.Address.Hex(), -tokenCreditAmount)
		if err != nil {
			errGrp.Add(Error.New("unable to create token payment billing transaction for user %s", wallet.UserID.String()))
			continue
		}

		creditNoteID, err := service.addCreditNoteToInvoice(ctx, invoice.ID, cusID, wallet.Address.Hex(), tokenCreditAmount, txID)
		if err != nil {
			// attempt to fail any pending transactions
			err := service.billingDB.FailPendingInvoiceTokenPayments(ctx, txID)
			if err != nil {
				errGrp.Add(Error.New("unable to fail the pending transactions for user %s", wallet.UserID.String()))
			}
			errGrp.Add(Error.New("unable to create token payment credit note for user %s", wallet.UserID.String()))
			continue
		}

		metadata, err := json.Marshal(map[string]interface{}{
			"Credit Note ID": creditNoteID,
		})

		if err != nil {
			// attempt to fail any pending transactions
			err := service.billingDB.FailPendingInvoiceTokenPayments(ctx, txID)
			if err != nil {
				errGrp.Add(Error.New("unable to fail the pending transactions for user %s", wallet.UserID.String()))
			}
			errGrp.Add(Error.New("unable to marshall credit note ID %s", creditNoteID))
			continue
		}

		err = service.billingDB.UpdateMetadata(ctx, txID, metadata)
		if err != nil {
			// attempt to fail any pending transactions
			err := service.billingDB.FailPendingInvoiceTokenPayments(ctx, txID)
			if err != nil {
				errGrp.Add(Error.New("unable to fail the pending transactions for user %s", wallet.UserID.String()))
			}
			errGrp.Add(Error.New("unable to add credit note ID to billing transaction for user %s", wallet.UserID.String()))
			continue
		}

		err = service.billingDB.CompletePendingInvoiceTokenPayments(ctx, txID)
		if err != nil {
			// attempt to fail any pending transactions
			err := service.billingDB.FailPendingInvoiceTokenPayments(ctx, txID)
			if err != nil {
				errGrp.Add(Error.New("unable to fail the pending transactions for user %s", wallet.UserID.String()))
			}
			errGrp.Add(Error.New("unable to update status for billing transaction for user %s", wallet.UserID.String()))
			continue
		}
	}
	return errGrp.Err()
}

// mustSkipUser checks whether a user does not have a status of console.Active or is in Free tier.
func (service *Service) mustSkipUser(ctx context.Context, userID uuid.UUID) (bool, error) {
	user, err := service.usersDB.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, Error.New("unable to look up user %s: %w", userID, err)
	}

	if service.enableFreeTrialLogic {
		return user.Status != console.Active || !user.PaidTier, nil
	}
	return user.Status != console.Active, nil
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
func (service *Service) calculateProjectUsagePrice(usage accounting.ProjectUsage, pricing payments.ProjectUsagePriceModel) projectUsagePrice {
	return projectUsagePrice{
		Storage:  pricing.StorageMBMonthCents.Mul(storageMBMonthDecimal(usage.Storage)).Round(0),
		Egress:   pricing.EgressMBCents.Mul(egressMBDecimal(usage.Egress)).Round(0),
		Segments: pricing.SegmentMonthCents.Mul(segmentMonthDecimal(usage.SegmentCount)).Round(0),
	}
}

// SetNow allows tests to have the Service act as if the current time is whatever
// they want. This avoids races and sleeping, making tests more reliable and efficient.
func (service *Service) SetNow(now func() time.Time) {
	service.nowFn = now
}

// getFromDate returns from date value used for data usage calculations depending on users upgrade time.
func (service *Service) getFromDate(ctx context.Context, userID uuid.UUID, start, end time.Time) (time.Time, error) {
	upgradeTime, err := service.usersDB.GetUpgradeTime(ctx, userID)
	if err != nil {
		return start, err
	}

	if upgradeTime == nil {
		return start, nil
	}

	utc := upgradeTime.UTC()
	dayAfterUpgrade := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)

	if dayAfterUpgrade.After(start) && dayAfterUpgrade.Before(end) {
		return dayAfterUpgrade, nil
	}

	return start, nil
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

// doesProjectRecordHaveNoUsage returns true if the given project record
// represents a billing cycle where there was no usage.
func doesProjectRecordHaveNoUsage(record ProjectRecord) bool {
	return record.Storage == 0 && record.Egress == 0 && record.Segments == 0
}

// applyEgressDiscount returns the amount of egress that we should charge for by subtracting
// the discounted amount.
func applyEgressDiscount(usage accounting.ProjectUsage, model payments.ProjectUsagePriceModel) int64 {
	egress := usage.Egress - int64(math.Round(usage.Storage/hoursPerMonth*model.EgressDiscountRatio))
	if egress < 0 {
		egress = 0
	}
	return egress
}

// Healthy returns true if this service can contact stripe.
func (service *Service) Healthy(ctx context.Context) bool {
	listParam := stripe.CustomerListParams{
		ListParams: stripe.ListParams{
			Context: ctx, Limit: stripe.Int64(1),
		},
	}

	return service.stripeClient.Customers().List(&listParam).Err() == nil
}

// Name returns the name of this service.
func (service *Service) Name() string {
	return "stripeService"
}
