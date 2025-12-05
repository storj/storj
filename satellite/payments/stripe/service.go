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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/currency"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/private/healthcheck"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
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
	// hoursPerMonth is the number of months in a billing month. For the purpose of billing, a byte*month's month is always 30 days.
	hoursPerMonth = 24 * 30

	// mbToGBConversionFactor is the factor used to convert MB units to GB units.
	// Since 1 GB = 1000 MB (using decimal notation for billing), we multiply prices
	// by this factor and divide quantities by this factor when converting from MB to GB.
	mbToGBConversionFactor = 1000

	segmentInvoiceItemDesc = " - Segment Fee (Segment-Month)"
)

// ServiceDependencies consolidates all database and service dependencies for stripe.NewService.
type ServiceDependencies struct {
	DB           DB
	WalletsDB    storjscan.WalletsDB
	BillingDB    billing.TransactionsDB
	ProjectsDB   console.Projects
	UsersDB      console.Users
	UsageDB      accounting.ProjectAccounting
	Analytics    *analytics.Service
	Emission     *emission.Service
	Entitlements *entitlements.Service
}

// PricingConfig consolidates all pricing-related configuration for stripe.NewService.
type PricingConfig struct {
	UsagePrices         payments.ProjectUsagePriceModel
	UsagePriceOverrides map[string]payments.ProjectUsagePriceModel
	ProductPriceMap     map[int32]payments.ProductUsagePriceModel
	PartnerPlacementMap payments.PartnersPlacementProductMap
	PlacementProductMap payments.PlacementProductIdMap
	PackagePlans        map[string]payments.PackagePlan
	BonusRate           int64
	MinimumChargeAmount int64
	MinimumChargeDate   *time.Time
}

// ServiceConfig consolidates various service configuration flags for stripe.NewService.
type ServiceConfig struct {
	DeleteAccountEnabled       bool
	DeleteProjectCostThreshold int64
	EntitlementsEnabled        bool
}

// Service is an implementation for payment service via Stripe and Coinpayments.
//
// architecture: Service
type Service struct {
	log          *zap.Logger
	stripeClient Client

	db           DB
	walletsDB    storjscan.WalletsDB
	billingDB    billing.TransactionsDB
	projectsDB   console.Projects
	usersDB      console.Users
	usageDB      accounting.ProjectAccounting
	analytics    *analytics.Service
	emission     *emission.Service
	entitlements *entitlements.Service

	config        ServiceConfig
	stripeConfig  Config
	pricingConfig PricingConfig

	// partnerNames is a list of partner names that may appear as bucket "user agent", and are explicitly associated with custom pricing.
	// If a bucket has a "partner"/"user agent" that does not appear in this list, it is treated as "unpartnered usage" from a billing perspective.
	partnerNames []string

	nowFn func() time.Time
}

// NewService creates a Service instance.
func NewService(
	log *zap.Logger,
	stripeClient Client,
	deps ServiceDependencies,
	config ServiceConfig,
	stripeConfig Config,
	pricing PricingConfig,
) (*Service, error) {
	var partners []string
	addedPartners := make(map[string]struct{})
	// partners relevant to billing may be defined as part of `usagePriceOverrides`, or `partnerPlacementMap`. Eventually, `usagePriceOverrides` will become legacy, and be replaced with `partnerPlacementMap`.
	for partner := range pricing.UsagePriceOverrides {
		if _, ok := addedPartners[partner]; ok {
			continue
		}
		partners = append(partners, partner)
		addedPartners[partner] = struct{}{}
	}
	for partner := range pricing.PartnerPlacementMap {
		if _, ok := addedPartners[partner]; ok {
			continue
		}
		partners = append(partners, partner)
		addedPartners[partner] = struct{}{}
	}

	return &Service{
		log:          log,
		stripeClient: stripeClient,

		db:           deps.DB,
		walletsDB:    deps.WalletsDB,
		billingDB:    deps.BillingDB,
		projectsDB:   deps.ProjectsDB,
		usersDB:      deps.UsersDB,
		usageDB:      deps.UsageDB,
		analytics:    deps.Analytics,
		emission:     deps.Emission,
		entitlements: deps.Entitlements,

		config:        config,
		pricingConfig: pricing,
		stripeConfig:  stripeConfig,

		partnerNames: partners,

		nowFn: time.Now,
	}, nil
}

// Accounts exposes all needed functionality to manage payment accounts.
func (service *Service) Accounts() payments.Accounts {
	return &accounts{service: service}
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
	customersPage := CustomersPage{
		Next: true,
	}

	for customersPage.Next {
		if err = ctx.Err(); err != nil {
			return Error.Wrap(err)
		}

		customersPage, err = service.db.Customers().List(ctx, customersPage.Cursor, service.stripeConfig.ListingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}
		numberOfCustomers += len(customersPage.Customers)

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
	var recordsToCreate []CreateProjectRecord

	for _, customer := range customers {
		ignore := service.ignoreNoStripeCustomer(ctx, customer.ID)
		if ignore {
			continue
		}

		if _, skip, err := service.mustSkipUser(ctx, customer.UserID); err != nil {
			return 0, Error.New("unable to determine if user must be skipped: %w", err)
		} else if skip {
			continue
		}

		// We include only active projects in the invoice.
		projects, err := service.projectsDB.GetOwnActive(ctx, customer.UserID)
		if err != nil {
			return 0, Error.New("unable to get own projects: %w", err)
		}

		records, err := service.createProjectRecords(ctx, &customer, projects, start, end)
		if err != nil {
			return 0, Error.New("unable to create project records: %w", err)
		}

		recordsToCreate = append(recordsToCreate, records...)
	}

	count := len(recordsToCreate)
	if count > 0 {
		err := service.db.ProjectRecords().Create(ctx, recordsToCreate, start, end)
		if err != nil {
			return 0, Error.New("failed to create regular project records: %w", err)
		}
	}

	return count, nil
}

// If the customer does not exist in stripe, we skip it.
// This is a workaround for the issue with missing customers in stripe for QA stellite.
func (service *Service) ignoreNoStripeCustomer(ctx context.Context, customerID string) bool {
	if !service.stripeConfig.SkipNoCustomer {
		return false
	}

	_, err := service.stripeClient.Customers().Get(customerID, &stripe.CustomerParams{Params: stripe.Params{Context: ctx}})
	return err != nil
}

// createProjectRecords creates invoice project record if none exists.
func (service *Service) createProjectRecords(ctx context.Context, customer *Customer, projects []console.Project, start, end time.Time) (_ []CreateProjectRecord, err error) {
	defer mon.Task()(&ctx)(&err)

	var records []CreateProjectRecord
	for _, project := range projects {
		if err = ctx.Err(); err != nil {
			return nil, err
		}

		// This is unlikely to happen but still.
		if project.Status != nil && *project.Status == console.ProjectDisabled {
			service.log.Warn("Skipping disabled project.", zap.String("Customer ID", customer.ID), zap.String("Project ID", project.ID.String()))
			continue
		}

		if err = service.db.ProjectRecords().Check(ctx, project.ID, start, end); err != nil {
			if errors.Is(err, ErrProjectRecordExists) {
				service.log.Warn("Record for this project already exists.", zap.String("Customer ID", customer.ID), zap.String("Project ID", project.ID.String()))
				continue
			}

			return nil, err
		}

		from, to, err := service.getFromToDates(ctx, customer.UserID, start, end)
		if err != nil {
			return nil, err
		}

		usage, err := service.usageDB.GetProjectTotal(ctx, project.ID, from, to)
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

	limiter := sync2.NewLimiter(service.stripeConfig.MaxParallelCalls)
	defer func() {
		limiter.Wait()
	}()

	customersPage := CustomersPage{
		Next: true,
	}

	for customersPage.Next {
		customersPage, err = service.db.Customers().List(ctx, customersPage.Cursor, service.stripeConfig.ListingLimit, end)
		if err != nil {
			return err
		}
		for _, c := range customersPage.Customers {
			c := c

			limiter.Go(ctx, func() {
				ignore := service.ignoreNoStripeCustomer(ctx, c.ID)
				if ignore {
					return
				}

				_, skip, err := service.mustSkipUser(ctx, c.UserID)
				if err != nil {
					addErr(&mu, err)
					return
				}
				if skip {
					totalSkipped.Add(1)
					return
				}
				projects, err := service.projectsDB.GetOwnActive(ctx, c.UserID)
				if err != nil {
					addErr(&mu, err)
					return
				}

				projectIDs := []uuid.UUID{}
				publicIDMap := make(map[uuid.UUID]uuid.UUID)
				for _, p := range projects {
					projectIDs = append(projectIDs, p.ID)
					publicIDMap[p.ID] = p.PublicID
				}

				records, err := service.db.ProjectRecords().GetUnappliedByProjectIDs(ctx, projectIDs, start, end)
				if err != nil {
					addErr(&mu, err)
					return
				}

				// Create structures to aggregate all usage by product ID.
				// Those maps are mutated per record.
				productUsages := make(map[int32]accounting.ProjectUsage)
				productInfos := make(map[int32]payments.ProductUsagePriceModel)

				from, to, err := service.getFromToDates(ctx, c.UserID, start, end)
				if err != nil {
					addErr(&mu, err)
					return
				}

				for _, r := range records {
					totalRecords.Add(1)

					r.ProjectPublicID = publicIDMap[r.ProjectID]

					skipped, err := service.ProcessRecord(ctx, r, productUsages, productInfos, from, to)
					if err != nil {
						service.log.Error("ProcessRecord failed, records will not be consumed",
							zap.String("customer_id", c.ID),
							zap.String("project_id", r.ProjectID.String()),
							zap.Error(err))
						addErr(&mu, err)
						return
					}
					if skipped {
						totalSkipped.Add(1)
					}
				}

				items := service.InvoiceItemsFromTotalProjectUsages(productUsages, productInfos, period)
				// Stripe allows 250 items per invoice.
				// We should not have more than 248 new items.
				// 1 is reserved for the unpaid usage from previous billing cycle.
				// 1 is reserved for minimum charge item.
				if len(items) > 248 {
					addErr(&mu, Error.New("too many invoice items for customer %s", c.ID))
					return
				}

				for _, item := range items {
					item.Params = stripe.Params{Context: ctx}
					item.Currency = stripe.String(string(stripe.CurrencyUSD))
					item.Customer = stripe.String(c.ID)
					item.Period = &stripe.InvoiceItemPeriodParams{
						End:   stripe.Int64(to.Unix()),
						Start: stripe.Int64(from.Unix()),
					}

					_, err := service.stripeClient.InvoiceItems().New(item)
					if err != nil {
						addErr(&mu, err)
						return
					}
				}

				for _, r := range records {
					if err = service.db.ProjectRecords().Consume(ctx, r.ID); err != nil {
						addErr(&mu, err)
						return
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
			if service.stripeConfig.SkipNoCustomer && errors.Is(err, ErrNoCustomer) {
				continue
			}

			errGrp.Add(Error.New("unable to get stripe customer ID for user ID %s", wallet.UserID.String()))
			continue
		}

		ignore := service.ignoreNoStripeCustomer(ctx, customerID)
		if ignore {
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

// ProcessRecord processes record and mutates overall customer usages.
// It is only used if product-based invoicing is enabled.
// Exported for testing.
func (service *Service) ProcessRecord(
	ctx context.Context,
	record ProjectRecord,
	productUsages map[int32]accounting.ProjectUsage,
	productInfos map[int32]payments.ProductUsagePriceModel,
	from, to time.Time,
) (skipped bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if service.stripeConfig.SkipEmptyInvoices && doesProjectRecordHaveNoUsage(record) {
		// TODO: should we consider this as skipped?
		return true, nil
	}

	err = service.getAndProcessUsages(ctx, record.ProjectID, record.ProjectPublicID, productUsages, productInfos, from, to)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (service *Service) getAndProcessUsages(
	ctx context.Context,
	projectID, projectPublicID uuid.UUID,
	productUsages map[int32]accounting.ProjectUsage,
	productInfos map[int32]payments.ProductUsagePriceModel,
	from, to time.Time,
) error {
	usages, err := service.usageDB.GetProjectTotalByPartnerAndPlacement(ctx, projectID, service.partnerNames, from, to, false)
	if err != nil {
		return err
	}

	// Process each partner/placement usage entry.
	for key, usage := range usages {
		if key == "" {
			return errs.New("invalid usage key format")
		}

		productID, priceModel := service.productIdAndPriceForUsageKey(ctx, projectPublicID, key)

		// Create or update the product usage entry.
		if existingUsage, ok := productUsages[productID]; ok {
			// Add to existing usage.
			existingUsage.Storage += usage.Storage
			existingUsage.Egress += usage.Egress
			existingUsage.SegmentCount += usage.SegmentCount
			productUsages[productID] = existingUsage
		} else {
			// Initialize with this usage.
			productUsages[productID] = usage

			// Get product name and SKU.
			var (
				productName string
				storageSKU  string
				egressSKU   string
				segmentSKU  string
			)
			if product, ok := service.pricingConfig.ProductPriceMap[productID]; ok {
				productName = product.ProductName
				storageSKU = product.StorageSKU
				egressSKU = product.EgressSKU
				segmentSKU = product.SegmentSKU
			} else {
				service.log.Error("failed to get product for ID", zap.Int("productID", int(productID)))
				// fall back to  "Product x" as the name for an "unknown" product.
				productName = fmt.Sprintf("Product %d", productID)
			}

			// Initialize product info.
			productInfos[productID] = payments.ProductUsagePriceModel{
				ProductID:                productID,
				ProductName:              productName,
				StorageSKU:               storageSKU,
				EgressSKU:                egressSKU,
				SegmentSKU:               segmentSKU,
				SmallObjectFeeCents:      priceModel.SmallObjectFeeCents,
				MinimumRetentionFeeCents: priceModel.MinimumRetentionFeeCents,
				SmallObjectFeeSKU:        priceModel.SmallObjectFeeSKU,
				MinimumRetentionFeeSKU:   priceModel.MinimumRetentionFeeSKU,
				EgressOverageMode:        priceModel.EgressOverageMode,
				IncludedEgressSKU:        priceModel.IncludedEgressSKU,
				ProjectUsagePriceModel:   priceModel.ProjectUsagePriceModel,
				UseGBUnits:               priceModel.UseGBUnits,
			}
		}
	}

	return nil
}

func (service *Service) productIdAndPriceForUsageKey(ctx context.Context, projectPublicID uuid.UUID, key string) (int32, payments.ProductUsagePriceModel) {
	partner := ""
	placement := int(storj.DefaultPlacement)

	// Split the key to extract partner and placement.
	parts := strings.Split(key, "|")
	if len(parts) >= 1 {
		partner = parts[0]
	}
	if len(parts) >= 2 {
		placement64, err := strconv.ParseInt(parts[1], 10, 32)
		if err == nil {
			placement = int(placement64)
		}
	}

	// Get price model for the partner and placement.
	return service.Accounts().GetPartnerPlacementPriceModel(ctx, projectPublicID, partner, storj.PlacementConstraint(placement))
}

// InvoiceItemsFromTotalProjectUsages calculates per-product Stripe invoice items from total project usages.
// Exported for testing.
func (service *Service) InvoiceItemsFromTotalProjectUsages(productUsages map[int32]accounting.ProjectUsage, productInfos map[int32]payments.ProductUsagePriceModel, period time.Time) (result []*stripe.InvoiceItemParams) {
	productIDs := getSortedProductIDs(productUsages)

	// Generate invoice items from aggregated product usage.
	for _, productID := range productIDs {
		usage := productUsages[productID]
		info := productInfos[productID]
		prefix := info.ProductName
		productIDStr := strconv.Itoa(int(productID))

		// Calculate egress discount.
		discountedUsage := usage.Clone()
		discountedUsage.Egress = applyEgressDiscount(usage, info.ProjectUsagePriceModel)

		// Create storage invoice item.
		storageItem := &stripe.InvoiceItemParams{}
		var storageDesc string
		if info.UseGBUnits {
			storageDesc = prefix + " - Storage (GB-Month)"
		} else {
			storageDesc = prefix + " - Storage (MB-Month)"
		}
		if info.StorageSKU != "" && service.stripeConfig.SkuEnabled {
			storageItem.AddMetadata("SKU", info.StorageSKU)
			if service.stripeConfig.InvItemSKUInDescription {
				storageDesc += " - " + info.StorageSKU
			}
		}
		storageItem.Description = stripe.String(storageDesc)
		if info.UseGBUnits {
			// New products: convert from byte-hours to GB-Month.
			// storage (byte-hours) / 1e6 / mbToGBConversionFactor / hoursPerMonth = GB-Month
			storageAdjustedMonth := decimal.NewFromFloat(discountedUsage.Storage).Shift(-6).Div(decimal.NewFromInt(mbToGBConversionFactor)).Div(decimal.NewFromInt(hoursPerMonth))
			var storageQuantity int64
			if service.stripeConfig.RoundUpInvoiceUsage {
				storageQuantity = storageAdjustedMonth.Ceil().IntPart()
				// Ensure at least 1 unit if there's any storage usage (even if it rounds to 0).
				if discountedUsage.Storage > 0 && storageQuantity == 0 {
					storageQuantity = 1
				}
			} else {
				storageQuantity = storageAdjustedMonth.Round(0).IntPart()
			}
			storageItem.Quantity = stripe.Int64(storageQuantity)
			// Multiply price by mbToGBConversionFactor to convert from MB cents to GB cents.
			storagePrice, _ := info.ProjectUsagePriceModel.StorageMBMonthCents.Mul(decimal.NewFromInt(mbToGBConversionFactor)).Float64()
			storageItem.UnitAmountDecimal = stripe.Float64(storagePrice)
		} else {
			// Legacy products: use MB-Month with rounding.
			storageMBMonth := storageMBMonthDecimal(discountedUsage.Storage)
			storageItem.Quantity = stripe.Int64(storageMBMonth.IntPart())
			storagePrice, _ := info.ProjectUsagePriceModel.StorageMBMonthCents.Float64()
			storageItem.UnitAmountDecimal = stripe.Float64(storagePrice)
		}
		if service.stripeConfig.UseIdempotency {
			storageItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "storage", period))
		}

		result = append(result, storageItem)

		// Create egress invoice item(s).
		if info.EgressOverageMode {
			// In overage mode, show both included egress (at $0) and overage (when present).

			var totalEgressQuantity, overageEgressQuantity, includedEgressQuantity int64
			var egressUnitDesc string

			if info.UseGBUnits {
				// New products: convert from bytes to GB.
				// egress (bytes) / 1e6 / mbToGBConversionFactor = GB
				totalEgressAdjusted := decimal.NewFromInt(usage.Egress).Shift(-6).Div(decimal.NewFromInt(mbToGBConversionFactor))
				overageEgressAdjusted := decimal.NewFromInt(discountedUsage.Egress).Shift(-6).Div(decimal.NewFromInt(mbToGBConversionFactor))

				if service.stripeConfig.RoundUpInvoiceUsage {
					totalEgressQuantity = totalEgressAdjusted.Ceil().IntPart()
					overageEgressQuantity = overageEgressAdjusted.Ceil().IntPart()

					// Ensure at least 1 unit if there's any egress usage (even if it rounds to 0).
					if usage.Egress > 0 && totalEgressQuantity == 0 {
						totalEgressQuantity = 1
					}
					if discountedUsage.Egress > 0 && overageEgressQuantity == 0 {
						overageEgressQuantity = 1
					}
				} else {
					totalEgressQuantity = totalEgressAdjusted.Round(0).IntPart()
					overageEgressQuantity = overageEgressAdjusted.Round(0).IntPart()
				}

				includedEgressQuantity = totalEgressQuantity - overageEgressQuantity
				egressUnitDesc = "GB"
			} else {
				// Legacy products: use MB with rounding.
				totalEgressMB := egressMBDecimal(usage.Egress)
				overageEgressMB := egressMBDecimal(discountedUsage.Egress)
				totalEgressQuantity = totalEgressMB.IntPart()
				overageEgressQuantity = overageEgressMB.IntPart()
				includedEgressQuantity = totalEgressQuantity - overageEgressQuantity
				egressUnitDesc = "MB"
			}

			if includedEgressQuantity > 0 {
				includedEgressItem := &stripe.InvoiceItemParams{}

				// Format discount ratio for description (e.g., "3X" for ratio 3.0, "0.5X" for 0.5).
				discountRatio := info.ProjectUsagePriceModel.EgressDiscountRatio
				var discountRatioStr string
				if discountRatio == float64(int64(discountRatio)) {
					// Whole number, format without decimal places.
					discountRatioStr = fmt.Sprintf("%.0fX", discountRatio)
				} else {
					// Has decimal places, show with appropriate precision.
					discountRatioStr = fmt.Sprintf("%.1fX", discountRatio)
				}
				includedEgressDesc := prefix + fmt.Sprintf(" - %s Included Egress (%s)", discountRatioStr, egressUnitDesc)
				if info.IncludedEgressSKU != "" && service.stripeConfig.SkuEnabled {
					includedEgressItem.AddMetadata("SKU", info.IncludedEgressSKU)
					if service.stripeConfig.InvItemSKUInDescription {
						includedEgressDesc += " - " + info.IncludedEgressSKU
					}
				}
				includedEgressItem.Description = stripe.String(includedEgressDesc)
				includedEgressItem.Quantity = stripe.Int64(includedEgressQuantity)
				includedEgressItem.UnitAmountDecimal = stripe.Float64(0) // $0 price for included egress.
				if service.stripeConfig.UseIdempotency {
					includedEgressItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "egress-included", period))
				}

				result = append(result, includedEgressItem)
			}

			if overageEgressQuantity > 0 {
				overageEgressItem := &stripe.InvoiceItemParams{}
				overageEgressDesc := prefix + fmt.Sprintf(" - Additional Egress (%s)", egressUnitDesc)

				if info.EgressSKU != "" && service.stripeConfig.SkuEnabled {
					overageEgressItem.AddMetadata("SKU", info.EgressSKU)
					if service.stripeConfig.InvItemSKUInDescription {
						overageEgressDesc += " - " + info.EgressSKU
					}
				}
				overageEgressItem.Description = stripe.String(overageEgressDesc)
				overageEgressItem.Quantity = stripe.Int64(overageEgressQuantity)
				if info.UseGBUnits {
					// New products: multiply price by mbToGBConversionFactor to convert from MB cents to GB cents.
					egressPrice, _ := info.ProjectUsagePriceModel.EgressMBCents.Mul(decimal.NewFromInt(mbToGBConversionFactor)).Float64()
					overageEgressItem.UnitAmountDecimal = stripe.Float64(egressPrice)
				} else {
					// Legacy products: use price as-is.
					egressPrice, _ := info.ProjectUsagePriceModel.EgressMBCents.Float64()
					overageEgressItem.UnitAmountDecimal = stripe.Float64(egressPrice)
				}
				if service.stripeConfig.UseIdempotency {
					overageEgressItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "egress-overage", period))
				}

				result = append(result, overageEgressItem)
			}
		} else {
			egressItem := &stripe.InvoiceItemParams{}
			var egressDesc string
			if info.UseGBUnits {
				egressDesc = prefix + " - Egress Bandwidth (GB)"
			} else {
				egressDesc = prefix + " - Egress Bandwidth (MB)"
			}
			if info.EgressSKU != "" && service.stripeConfig.SkuEnabled {
				egressItem.AddMetadata("SKU", info.EgressSKU)
				if service.stripeConfig.InvItemSKUInDescription {
					egressDesc += " - " + info.EgressSKU
				}
			}
			egressItem.Description = stripe.String(egressDesc)
			if info.UseGBUnits {
				// New products: convert from bytes to GB.
				// Avoid intermediate MB rounding to preserve precision.
				// egress (bytes) / 1e6 / mbToGBConversionFactor = GB
				egressAdjusted := decimal.NewFromInt(discountedUsage.Egress).Shift(-6).Div(decimal.NewFromInt(mbToGBConversionFactor))
				var egressQuantity int64
				if service.stripeConfig.RoundUpInvoiceUsage {
					egressQuantity = egressAdjusted.Ceil().IntPart()
					// Ensure at least 1 unit if there's any egress usage (even if it rounds to 0).
					if discountedUsage.Egress > 0 && egressQuantity == 0 {
						egressQuantity = 1
					}
				} else {
					egressQuantity = egressAdjusted.Round(0).IntPart()
				}
				egressItem.Quantity = stripe.Int64(egressQuantity)
				// Multiply price by mbToGBConversionFactor to convert from MB cents to GB cents.
				egressPrice, _ := info.ProjectUsagePriceModel.EgressMBCents.Mul(decimal.NewFromInt(mbToGBConversionFactor)).Float64()
				egressItem.UnitAmountDecimal = stripe.Float64(egressPrice)
			} else {
				// Legacy products: use MB with rounding.
				egressMB := egressMBDecimal(discountedUsage.Egress)
				egressItem.Quantity = stripe.Int64(egressMB.IntPart())
				egressPrice, _ := info.ProjectUsagePriceModel.EgressMBCents.Float64()
				egressItem.UnitAmountDecimal = stripe.Float64(egressPrice)
			}
			if service.stripeConfig.UseIdempotency {
				egressItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "egress", period))
			}

			result = append(result, egressItem)
		}

		// Create segment invoice item.
		// Note: Segment fees are not affected by UseGBUnits, they use the same units for all products.
		if !info.ProjectUsagePriceModel.SegmentMonthCents.IsZero() {
			segmentItem := &stripe.InvoiceItemParams{}
			segmentDesc := prefix + segmentInvoiceItemDesc
			if info.SegmentSKU != "" && service.stripeConfig.SkuEnabled {
				segmentItem.AddMetadata("SKU", info.SegmentSKU)
				if service.stripeConfig.InvItemSKUInDescription {
					segmentDesc += " - " + info.SegmentSKU
				}
			}
			segmentItem.Description = stripe.String(segmentDesc)
			segmentItem.Quantity = stripe.Int64(segmentMonthDecimal(discountedUsage.SegmentCount).IntPart())
			segmentPrice, _ := info.ProjectUsagePriceModel.SegmentMonthCents.Float64()
			segmentItem.UnitAmountDecimal = stripe.Float64(segmentPrice)
			if service.stripeConfig.UseIdempotency {
				segmentItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "segment", period))
			}

			result = append(result, segmentItem)
		}

		if !info.SmallObjectFeeCents.IsZero() {
			smallObjectFeeItem := &stripe.InvoiceItemParams{}
			var smallObjectFeeDesc string
			if info.UseGBUnits {
				smallObjectFeeDesc = prefix + " - Minimum Object Size Remainder (GB-Month)"
			} else {
				smallObjectFeeDesc = prefix + " - Minimum Object Size Remainder (MB-Month)"
			}
			smallObjectFeeItem.Description = stripe.String(smallObjectFeeDesc)
			smallObjectFeeItem.Quantity = stripe.Int64(0) // not applied for now.
			if info.UseGBUnits {
				// Multiply price by mbToGBConversionFactor to convert from MB cents to GB cents.
				smallObjectFeePrice, _ := info.SmallObjectFeeCents.Mul(decimal.NewFromInt(mbToGBConversionFactor)).Float64()
				smallObjectFeeItem.UnitAmountDecimal = stripe.Float64(smallObjectFeePrice)
			} else {
				smallObjectFeePrice, _ := info.SmallObjectFeeCents.Float64()
				smallObjectFeeItem.UnitAmountDecimal = stripe.Float64(smallObjectFeePrice)
			}
			if info.SmallObjectFeeSKU != "" && service.stripeConfig.SkuEnabled {
				smallObjectFeeItem.AddMetadata("SKU", info.SmallObjectFeeSKU)
			}
			if service.stripeConfig.UseIdempotency {
				smallObjectFeeItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "small-object-fee", period))
			}

			result = append(result, smallObjectFeeItem)
		}

		if !info.MinimumRetentionFeeCents.IsZero() {
			minimumRetentionFeeItem := &stripe.InvoiceItemParams{}
			var minimumRetentionFeeDesc string
			if info.UseGBUnits {
				minimumRetentionFeeDesc = prefix + " - Minimum Storage Retention Remainder (GB-Month)"
			} else {
				minimumRetentionFeeDesc = prefix + " - Minimum Storage Retention Remainder (MB-Month)"
			}
			minimumRetentionFeeItem.Description = stripe.String(minimumRetentionFeeDesc)
			minimumRetentionFeeItem.Quantity = stripe.Int64(0) // not applied for now.
			if info.UseGBUnits {
				// Multiply price by mbToGBConversionFactor to convert from MB cents to GB cents.
				minimumRetentionFeePrice, _ := info.MinimumRetentionFeeCents.Mul(decimal.NewFromInt(mbToGBConversionFactor)).Float64()
				minimumRetentionFeeItem.UnitAmountDecimal = stripe.Float64(minimumRetentionFeePrice)
			} else {
				minimumRetentionFeePrice, _ := info.MinimumRetentionFeeCents.Float64()
				minimumRetentionFeeItem.UnitAmountDecimal = stripe.Float64(minimumRetentionFeePrice)
			}
			if info.MinimumRetentionFeeSKU != "" && service.stripeConfig.SkuEnabled {
				minimumRetentionFeeItem.AddMetadata("SKU", info.MinimumRetentionFeeSKU)
			}
			if service.stripeConfig.UseIdempotency {
				minimumRetentionFeeItem.SetIdempotencyKey(getPerProductIdempotencyKey(productIDStr, "minimum-retention-fee", period))
			}

			result = append(result, minimumRetentionFeeItem)
		}
	}

	service.log.Info("invoice items by product", zap.Any("result", result))
	return result
}

func getSortedProductIDs(productUsages map[int32]accounting.ProjectUsage) (productIDs []int32) {
	// Sort product IDs for consistent ordering.
	for productID := range productUsages {
		productIDs = append(productIDs, productID)
	}
	slices.Sort(productIDs)

	return productIDs
}

func getPerProductIdempotencyKey(productID, identifier string, period time.Time) string {
	key := fmt.Sprintf("%s-%s-%s", productID, identifier, period.Format("2006-01"))
	return strings.ToLower(strings.ReplaceAll(key, " ", "-"))
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
	var hubspotObjectID *string

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
			user, err := service.usersDB.Get(ctx, customer.UserID)
			if err == nil {
				hubspotObjectID = user.HubspotObjectID
			}

			service.analytics.TrackExpiredCreditNeedsRemoval(customer.UserID, customer.ID, *customer.PackagePlan, hubspotObjectID)
		}
		return true, nil
	}

	// If no other credit found, we can set the balance to zero.
	if balance < 0 {
		_, err = service.stripeClient.CustomerBalanceTransactions().New(&stripe.CustomerBalanceTransactionParams{
			Customer:    stripe.String(customer.ID),
			Amount:      stripe.Int64(-balance),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Description: stripe.String(*customer.PackagePlan + " expired"),
		})
		if err != nil {
			return false, Error.Wrap(err)
		}
		if service.analytics != nil {
			user, err := service.usersDB.Get(ctx, customer.UserID)
			if err == nil {
				hubspotObjectID = user.HubspotObjectID
			}

			service.analytics.TrackExpiredCreditRemoved(customer.UserID, customer.ID, *customer.PackagePlan, hubspotObjectID)
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

	limiter := sync2.NewLimiter(service.stripeConfig.MaxParallelCalls)
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
				if _, skip, err := service.mustSkipUser(ctx, c.UserID); err != nil {
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
		Coupon: stripe.String(service.stripeConfig.StripeFreeTierCouponID),
	}
	_, err = service.stripeClient.Customers().Update(cusID, params)
	if err != nil {
		service.log.Error("Failed to update customer with free tier coupon", zap.Error(err))
		return false, err
	}

	return true, nil
}

// CreateInvoices lists through all customers, removes expired credit if applicable, and creates invoices.
func (service *Service) CreateInvoices(ctx context.Context, period time.Time) (err error) {
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
		cusPage, err := service.db.Customers().List(ctx, nextCursor, service.stripeConfig.ListingLimit, end)
		if err != nil {
			return Error.Wrap(err)
		}

		if service.stripeConfig.RemoveExpiredCredit {
			for _, c := range cusPage.Customers {

				if c.PackagePlan != nil {
					ignore := service.ignoreNoStripeCustomer(ctx, c.ID)
					if ignore {
						continue
					}

					if _, err := service.RemoveExpiredPackageCredit(ctx, c); err != nil {
						return Error.Wrap(err)
					}
				}
			}
		}

		scheduled, draft, err := service.createInvoices(ctx, cusPage.Customers, start, end)
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

// CreateInvoice creates invoice for Stripe customer.
// Exported for testing.
func (service *Service) CreateInvoice(ctx context.Context, cusID string, user *console.User, start, end time.Time) (stripeInvoice *stripe.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	var (
		lastItemID   string
		totalStorage int64
		hasItems     bool
		hasInvoice   bool
		hasShortFall bool
	)

	minimumChargeDate := service.pricingConfig.MinimumChargeDate
	applyMinimumCharge := service.pricingConfig.MinimumChargeAmount > 0 && (minimumChargeDate == nil || !start.Before(*minimumChargeDate))

	if applyMinimumCharge {
		skip, err := service.Accounts().ShouldSkipMinimumCharge(ctx, cusID, user.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		if skip {
			applyMinimumCharge = false
		}
	}

	if applyMinimumCharge {
		// Edge case:
		// If some error happens while creating invoices, we should check if an invoice for this customer already exists.
		// If it does, we should not create a new one because this customer has already been processed.
		invoicesIterator := service.stripeClient.Invoices().List(&stripe.InvoiceListParams{
			ListParams: stripe.ListParams{Context: ctx, Limit: stripe.Int64(1)},
			Customer:   &cusID,
			Status:     stripe.String(string(stripe.InvoiceStatusDraft)),
			CreatedRange: &stripe.RangeQueryParams{
				GreaterThan: start.Unix(),
			},
		})

		for invoicesIterator.Next() {
			stripeInvoice = invoicesIterator.Invoice()
			hasInvoice = true
		}
		if err = invoicesIterator.Err(); err != nil {
			return nil, Error.Wrap(err)
		}
	}

	if !hasInvoice {
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
				if strings.Contains(item.Description, "Storage (MB-Month)") || strings.Contains(item.Description, "Storage (GB-Month)") {
					totalStorage += item.Quantity
				}

				lastItemID = item.ID
			}

			if err = itemsIter.Err(); err != nil {
				return nil, err
			}
			if service.stripeConfig.SkipEmptyInvoices && !hasItems {
				return nil, nil
			}

			// Use HasMore to determine if we should break the loop.
			if !itemsIter.List().GetListMeta().HasMore {
				break
			}
		}

		// Okay, this is a bit confusing. For the purposes of billing, the unit we
		// bill in is MB*months, where the month is a standard 30 day unit.
		// However, for the purposes of carbon impact, we actually care about the
		// real time line, and the average amount of bytes stored during that time.
		//
		// think about it this way - let's say a person has 1TB of data just sitting
		// in their account. in April, the person will use 1 TB*month, but in March,
		// that person will use 31/30 TB*month, and in February on a leap year, that
		// person will use 29/30 TB*month. (where again, above, the term "month"
		// means 30 days).
		//
		// for the carbon impact, in February, March, and April, we want to say the
		// person stored 1 TB. Not a varying amount of TB. And we want to say how
		// long the person stored the TB for (either 29 days, 30, or 31). So, we
		// need to care about the real month length, and the average amount of bytes
		// stored during that real month length.
		//
		// we'll start with the real month length:
		realTimeElapsed := end.Sub(start)
		// To make things "simpler", let's convert totalStorage from
		// MB*30days to MB*hours.
		totalStorageMBHours := float64(totalStorage) * hoursPerMonth
		// now, to figure out the average amount of MB used for a given time range,
		// we will divide the totalStorageMBHours by the real number of hours.
		realTimeElapsedHours := realTimeElapsed.Seconds() / (60 * 60)
		averageMB := totalStorageMBHours / realTimeElapsedHours

		// okay now we can calculate in a way that will be correct for february,
		// march, and april.
		impact, err := service.emission.CalculateImpact(&emission.CalculationInput{
			AmountOfDataInTB: averageMB / 1000 / 1000,
			Duration:         realTimeElapsed,
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
		footer := stripe.String(footerMsg)

		description := fmt.Sprintf("Storj Cloud Storage for %s %d", start.Month(), start.Year())

		stripeInvoice, err = service.stripeClient.Invoices().New(
			&stripe.InvoiceParams{
				Params:                      stripe.Params{Context: ctx},
				Customer:                    stripe.String(cusID),
				AutoAdvance:                 stripe.Bool(service.stripeConfig.AutoAdvance),
				Description:                 stripe.String(description),
				PendingInvoiceItemsBehavior: stripe.String("include"),
				Footer:                      footer,
			},
		)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	// Unlikely but still.
	if stripeInvoice == nil {
		return nil, Error.New("stripe invoice couldn't be generated for customer %s", cusID)
	}

	// We apply the minimum fee only if the invoice total is more than or equal to $0.01 and less than the minimum fee.
	if applyMinimumCharge && stripeInvoice.Total >= 1 && stripeInvoice.Total < service.pricingConfig.MinimumChargeAmount {
		shortfall := service.pricingConfig.MinimumChargeAmount - stripeInvoice.Total

		_, err = service.stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:      stripe.Params{Context: ctx},
			Customer:    stripe.String(cusID),
			Amount:      stripe.Int64(shortfall),
			Description: stripe.String("Minimum charge adjustment"),
			Currency:    stripe.String(string(stripe.CurrencyUSD)),
			Invoice:     stripe.String(stripeInvoice.ID),
			Period: &stripe.InvoiceItemPeriodParams{
				End:   stripe.Int64(end.Unix()),
				Start: stripe.Int64(start.Unix()),
			},
		})
		if err != nil {
			return nil, err
		}

		hasShortFall = true
	}

	// auto advance the invoice if nothing is due from the customer.
	if !stripeInvoice.AutoAdvance && stripeInvoice.Total == 0 && !hasShortFall {
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
func (service *Service) createInvoices(ctx context.Context, customers []Customer, start, end time.Time) (scheduled, draft int, err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(service.stripeConfig.MaxParallelCalls)
	var errGrp errs.Group
	var mu sync.Mutex

	for _, cus := range customers {
		cus := cus

		limiter.Go(ctx, func() {
			ignore := service.ignoreNoStripeCustomer(ctx, cus.ID)
			if ignore {
				return
			}

			user, skip, err := service.mustSkipUser(ctx, cus.UserID)
			if err != nil {
				mu.Lock()
				errGrp.Add(err)
				mu.Unlock()
				return
			} else if skip {
				return
			}

			inv, err := service.CreateInvoice(ctx, cus.ID, user, start, end)
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
			if service.stripeConfig.SkipNoCustomer && errs.Is(err, ErrNoCustomer) {
				continue
			}

			return err
		}

		if _, skip, err := service.mustSkipUser(ctx, userID); err != nil {
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
func (service *Service) GenerateInvoices(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("Preparing invoice project records")
	err = service.PrepareInvoiceProjectRecords(ctx, period)
	if err != nil {
		return err
	}

	service.log.Info("Applying invoice project records")
	err = service.InvoiceApplyProjectRecordsGrouped(ctx, period)
	if err != nil {
		return err
	}

	service.log.Info("Creating invoices")
	err = service.CreateInvoices(ctx, period)
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
		if _, skip, err := service.mustSkipUser(ctx, userID); err != nil {
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

		if service.config.DeleteAccountEnabled {
			user, err := service.usersDB.Get(ctx, userID)
			if err != nil {
				return Error.Wrap(err)
			}

			// we use line item's period.end field to check if it corresponds to user's status update at field.
			// unfortunately, we can't use invoice's period_end field as it's not relevant and can't be updated or predefined.
			if user.StatusUpdatedAt == nil || stripeInvoice.Lines.Data[0] == nil || stripeInvoice.Lines.Data[0].Period == nil {
				continue
			}

			statusUpdatedAt := user.StatusUpdatedAt.UTC().Unix()

			if user.Status == console.UserRequestedDeletion && !user.FinalInvoiceGenerated && stripeInvoice.Lines.Data[0].Period.End == statusUpdatedAt {
				invoiceGenerated := true
				err = service.usersDB.Update(ctx, user.ID, console.UpdateUserRequest{FinalInvoiceGenerated: &invoiceGenerated})
				if err != nil {
					return Error.Wrap(err)
				}
			}
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
	if _, skip, err := service.mustSkipUser(ctx, userID); err != nil {
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

// ListReusedCardFingerprints lists all reused credit card fingerprints across customers.
func (service *Service) ListReusedCardFingerprints(ctx context.Context) (list map[string]map[string]struct{}, err error) {
	defer mon.Task()(&ctx)(&err)

	list = make(map[string]map[string]struct{})

	params := &stripe.CustomerListParams{
		ListParams: stripe.ListParams{
			Context: ctx,
			Limit:   stripe.Int64(100),
		},
	}

	itr := service.stripeClient.Customers().List(params)
	for itr.Next() {
		cus := itr.Customer()

		userID, err := service.db.Customers().GetUserID(ctx, cus.ID)
		if err != nil {
			continue
		}

		if _, skip, err := service.mustSkipUser(ctx, userID); err != nil || skip {
			continue
		}

		cardParams := &stripe.PaymentMethodListParams{
			ListParams: stripe.ListParams{Context: ctx},
			Customer:   &cus.ID,
			Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
		}

		pmItr := service.stripeClient.PaymentMethods().List(cardParams)
		for pmItr.Next() {
			stripeCard := pmItr.PaymentMethod()

			if stripeCard == nil || stripeCard.Card == nil || stripeCard.Card.Fingerprint == "" {
				continue
			}

			if _, ok := list[stripeCard.Card.Fingerprint]; !ok {
				list[stripeCard.Card.Fingerprint] = make(map[string]struct{})
			}
			list[stripeCard.Card.Fingerprint][cus.ID] = struct{}{}
		}
		if err = pmItr.Err(); err != nil {
			return nil, Error.Wrap(err)
		}
	}
	if err = itr.Err(); err != nil {
		return nil, err
	}

	return list, nil
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

// mustSkipUser checks whether a user should be skipped based on their status and tier.
// It returns true if any of the following conditions are met:
// 1. The user has requested deletion and their final invoice has been generated.
// 2. The user's status is neither 'Active' nor 'UserRequestedDeletion'.
// 3. The user is not on a paid tier.
func (service *Service) mustSkipUser(ctx context.Context, userID uuid.UUID) (*console.User, bool, error) {
	user, err := service.usersDB.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, true, nil
		}
		return nil, false, Error.New("unable to look up user %s: %w", userID, err)
	}

	return user, (user.Status == console.UserRequestedDeletion && user.FinalInvoiceGenerated) ||
		(user.Status != console.Active && user.Status != console.UserRequestedDeletion) ||
		!user.IsPaid(), nil
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

// TotalInt64 returns int64 value of project usage price total.
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

// TestSetMinimumChargeCfg allows tests to set the minimum charge configuration.
func (service *Service) TestSetMinimumChargeCfg(amount int64, allUsersDate *time.Time) {
	service.pricingConfig.MinimumChargeAmount = amount
	service.pricingConfig.MinimumChargeDate = allUsersDate
}

// getFromToDates returns from/to date values used for data usage calculations depending on users upgrade time and status.
func (service *Service) getFromToDates(ctx context.Context, userID uuid.UUID, start, end time.Time) (time.Time, time.Time, error) {
	user, err := service.usersDB.Get(ctx, userID)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	from := start
	if user.UpgradeTime != nil {
		utc := user.UpgradeTime.UTC()
		dayAfterUpgrade := time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)

		if dayAfterUpgrade.After(start) && dayAfterUpgrade.Before(end) {
			from = dayAfterUpgrade
		}
	}

	to := end
	if service.config.DeleteAccountEnabled && user.Status == console.UserRequestedDeletion && user.StatusUpdatedAt != nil {
		statusUpdatedAt := user.StatusUpdatedAt.UTC()

		if !user.FinalInvoiceGenerated && statusUpdatedAt.Before(end) && statusUpdatedAt.After(start) {
			to = statusUpdatedAt
		}
	}

	return from, to, nil
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
