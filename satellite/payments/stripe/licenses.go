// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/payments"
)

// licenseSeatGroup identifies a set of license seats that are billed at the same
// prorated rate. Licenses that started at different points in the period accrue a
// different number of billable days, so they cannot share a single line item
// without misrepresenting quantity times unit price.
type licenseSeatGroup struct {
	productID  int32
	billedDays int
}

// licenseSeatInvoiceItemDesc is the description suffix for license seat line items.
const licenseSeatInvoiceItemDesc = " - Seats"

// maxInvoiceItems is how many items may be raised for one customer's invoice. Stripe
// allows 250, with one reserved for the previous cycle's unpaid usage and one for the
// minimum charge. Usage items and seat items land on the same invoice, so they share
// the budget rather than getting one each.
const maxInvoiceItems = 248

// LicenseSeatItemMetadataKey marks an invoice item as a license seat charge, and
// LicenseSeatItemMetadataValue is the value it is set to. Invoice generation reads this
// to tell seat charges apart from usage charges.
// LicenseSeatChargeMetadataKey marks which charge an invoice item was raised for.
const (
	LicenseSeatItemMetadataKey   = "ItemType"
	LicenseSeatItemMetadataValue = "license-seat"
	LicenseSeatChargeMetadataKey = "LicenseCharge"
)

func licenseSeatChargeKey(record LicenseRecord) string {
	return getPerProductIdempotencyKey(
		strconv.Itoa(int(record.ProductID)),
		fmt.Sprintf("license-%dd", record.BilledDays),
		record.PeriodStart,
	)
}

// CreateLicenseInvoiceItems creates pending invoice items in Stripe for account
// licenses that are billable in the given period. It must run before CreateInvoices
// so the items are swept into the customer's invoice for that period.
func (service *Service) CreateLicenseInvoiceItems(ctx context.Context, period time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !service.stripeConfig.PopulateLicenseInvoiceLineItem {
		service.log.Info("License invoice line items are disabled, skipping license invoice items")
		return nil
	}
	if !service.config.EntitlementsEnabled || service.entitlements == nil {
		service.log.Info("Entitlements are disabled, skipping license invoice items")
		return nil
	}

	now := service.nowFn().UTC()
	utc := period.UTC()

	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(utc.Year(), utc.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	if end.After(now) {
		return Error.New("allowed for past periods only")
	}

	var nextCursor uuid.UUID
	var totalCustomers, totalItems int
	var errGrp errs.Group
	for {
		cusPage, err := service.db.Customers().List(ctx, nextCursor, service.stripeConfig.ListingLimit, end)
		if err != nil {
			errGrp.Add(Error.Wrap(err))
			break
		}

		customers, items, err := service.createLicenseInvoiceItems(ctx, cusPage.Customers, start, end)
		errGrp.Add(err)
		totalCustomers += customers
		totalItems += items

		if !cusPage.Next {
			break
		}
		nextCursor = cusPage.Cursor
	}

	service.log.Info("Number of created license invoice items",
		zap.Int("customers", totalCustomers),
		zap.Int("items", totalItems),
	)
	return errGrp.Err()
}

// createLicenseInvoiceItems creates license invoice items for a page of customers.
func (service *Service) createLicenseInvoiceItems(ctx context.Context, customers []Customer, start, end time.Time) (billedCustomers, createdItems int, err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(service.stripeConfig.MaxParallelCalls)
	var errGrp errs.Group
	var mu sync.Mutex

	for _, cus := range customers {
		cus := cus

		limiter.Go(ctx, func() {
			items, err := service.createCustomerLicenseInvoiceItems(ctx, cus, start, end)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errGrp.Add(err)
				return
			}
			if items > 0 {
				billedCustomers++
				createdItems += items
			}
		})
	}

	limiter.Wait()

	return billedCustomers, createdItems, errGrp.Err()
}

// existingLicenseSeatCharges returns the charges the customer already has seat items for,
// so a charge raised by a run that then failed to consume its record is not raised again
// once Stripe has forgotten the idempotency key. Items already on an invoice count as
// much as pending ones for that.
//
// It also reports how many items are still pending, which is what the seats share the
// invoice budget with. Only items raised since the period began are counted, which
// covers the usage items raised earlier in the same run.
func (service *Service) existingLicenseSeatCharges(ctx context.Context, cusID string, periodStart time.Time) (_ map[string]struct{}, pending int, err error) {
	defer mon.Task()(&ctx)(&err)

	iter := service.stripeClient.InvoiceItems().List(&stripe.InvoiceItemListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   stripe.String(cusID),
		CreatedRange: &stripe.RangeQueryParams{
			GreaterThanOrEqual: periodStart.Unix(),
		},
	})

	charges := make(map[string]struct{})
	for iter.Next() {
		item := iter.InvoiceItem()
		// An item that is not on an invoice yet is one the seats have to share the
		// invoice budget with.
		if item.Invoice == nil {
			pending++
		}
		if item.Metadata[LicenseSeatItemMetadataKey] != LicenseSeatItemMetadataValue {
			continue
		}
		if key := item.Metadata[LicenseSeatChargeMetadataKey]; key != "" {
			charges[key] = struct{}{}
		}
	}
	if err = iter.Err(); err != nil {
		return nil, 0, Error.Wrap(err)
	}

	return charges, pending, nil
}

// createCustomerLicenseInvoiceItems creates the license invoice items for a single
// customer and returns how many were created.
//
// It runs in two phases. First the billable seat charges are recorded in the database.
// Then every record that has not been applied yet is turned into a Stripe invoice item
// and consumed.
//
// A charge whose record has been consumed is never raised again, nor is one whose item
// is already in Stripe. A charge is identified by its prorated rate, so a rerun that
// derives a different rate for a product raises that as a charge of its own.
func (service *Service) createCustomerLicenseInvoiceItems(ctx context.Context, cus Customer, start, end time.Time) (created int, err error) {
	defer mon.Task()(&ctx)(&err)

	if service.ignoreNoStripeCustomer(ctx, cus.ID) {
		return 0, nil
	}

	if _, skip, err := service.mustSkipUser(ctx, cus.UserID); err != nil {
		return 0, err
	} else if skip {
		return 0, nil
	}

	licenses, err := service.entitlements.Licenses().Get(ctx, cus.UserID)
	if err != nil {
		return 0, Error.New("unable to get licenses for user %s: %w", cus.UserID, err)
	}

	charges := service.LicenseSeatChargesFromLicenses(licenses.Licenses, cus.UserID, start, end)
	if err := service.prepareLicenseRecords(ctx, cus.UserID, charges, start, end); err != nil {
		return 0, err
	}

	records, err := service.db.LicenseRecords().ListUnappliedByUser(ctx, cus.UserID, start, end)
	if err != nil {
		return 0, Error.New("unable to list license records for user %s: %w", cus.UserID, err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	sortLicenseRecords(records)

	existing, pending, err := service.existingLicenseSeatCharges(ctx, cus.ID, start)
	if err != nil {
		return 0, err
	}

	toRaise := make([]LicenseRecord, 0, len(records))
	for _, record := range records {
		if _, ok := existing[licenseSeatChargeKey(record)]; ok {
			// Already raised by a run that did not consume it; catch the record up.
			if err := service.db.LicenseRecords().Consume(ctx, record.ID); err != nil {
				return created, Error.New("unable to consume license record %s: %w", record.ID, err)
			}
			continue
		}
		toRaise = append(toRaise, record)
	}

	// The usage items are already pending on the invoice these will join, so the seats
	// have to fit in what is left of the budget rather than in one of their own.
	if pending+len(toRaise) > maxInvoiceItems {
		return created, Error.New("too many invoice items for customer %s", cus.ID)
	}

	for _, record := range toRaise {
		item := service.InvoiceItemFromLicenseRecord(record)
		if item == nil {
			continue
		}

		PrepareInvoiceItemForCustomer(ctx, item, cus.ID, start, end)

		if _, err := service.stripeClient.InvoiceItems().New(item); err != nil {
			return created, Error.New("unable to create license invoice item for customer %s: %w", cus.ID, err)
		}

		if err := service.db.LicenseRecords().Consume(ctx, record.ID); err != nil {
			return created, Error.New("unable to consume license record %s: %w", record.ID, err)
		}
		created++
	}

	return created, nil
}

// prepareLicenseRecords records any seat charge for the period that is not recorded yet.
func (service *Service) prepareLicenseRecords(ctx context.Context, userID uuid.UUID, charges []CreateLicenseRecord, start, end time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	var toCreate []CreateLicenseRecord
	for _, charge := range charges {
		err := service.db.LicenseRecords().Check(ctx, userID, charge.ProductID, charge.BilledDays, start, end)
		if err != nil {
			if errs.Is(err, ErrLicenseRecordExists) {
				continue
			}
			return Error.New("unable to check license record for user %s: %w", userID, err)
		}
		toCreate = append(toCreate, charge)
	}

	if len(toCreate) == 0 {
		return nil
	}

	if err := service.db.LicenseRecords().Create(ctx, toCreate, start, end); err != nil {
		return Error.New("unable to create license records for user %s: %w", userID, err)
	}

	return nil
}

// LicenseSeatChargesFromLicenses computes the seat charges for the licenses that are
// billable in [start, end). Seats are aggregated per product and prorated rate so that
// a customer is charged once per distinct rate rather than once per license.
// Exported for testing.
func (service *Service) LicenseSeatChargesFromLicenses(licenses []entitlements.AccountLicense, userID uuid.UUID, start, end time.Time) (result []CreateLicenseRecord) {
	seats := make(map[licenseSeatGroup]int64)
	infos := make(map[int32]payments.ProductUsagePriceModel)
	// daysInPeriod is derived purely from start/end, so it is the same for every
	// license and only needs to be captured once.
	var daysInPeriod int

	for _, license := range licenses {
		// ProductID 0 is a free license and is never billed.
		if license.ProductID == 0 {
			continue
		}

		productID := int32(license.ProductID)
		info, ok := service.pricingConfig.ProductPriceMap[productID]
		if !ok {
			service.log.Warn("unknown product ID on license, skipping",
				zap.Stringer("user_id", userID),
				zap.Int32("product_id", productID),
			)
			continue
		}
		// Products without a seat price are not license products.
		if info.LicenseFeeCents.IsZero() {
			continue
		}

		billedDays, periodDays, ok := license.BillableSeatDays(start, end)
		if !ok {
			continue
		}
		if license.Count <= 0 {
			continue
		}
		daysInPeriod = periodDays

		group := licenseSeatGroup{productID: productID, billedDays: billedDays}
		seats[group] += int64(license.Count)
		infos[productID] = info
	}

	if daysInPeriod <= 0 {
		return nil
	}

	for _, group := range sortedLicenseSeatGroups(seats) {
		info := infos[group.productID]

		unitAmountCents := info.LicenseFeeCents.
			Mul(decimal.NewFromInt(int64(group.billedDays))).
			Div(decimal.NewFromInt(int64(daysInPeriod))).
			Round(0).
			IntPart()

		result = append(result, CreateLicenseRecord{
			UserID:          userID,
			ProductID:       group.productID,
			Seats:           seats[group],
			BilledDays:      group.billedDays,
			DaysInPeriod:    daysInPeriod,
			UnitAmountCents: unitAmountCents,
		})
	}

	return result
}

// InvoiceItemFromLicenseRecord builds the Stripe invoice item for a recorded seat
// charge. The amount comes from the record rather than from the licenses, so a recorded
// charge is billed at the amount it was prepared at; later license changes do not
// reprice it, though they can have a further charge prepared. Returns nil when the item
// cannot be built. Exported for testing.
func (service *Service) InvoiceItemFromLicenseRecord(record LicenseRecord) *stripe.InvoiceItemParams {
	info, ok := service.pricingConfig.ProductPriceMap[record.ProductID]
	if !ok {
		// The product was configured when the charge was recorded but is not now, so
		// there is no name or SKU to bill it under. Leave the record unapplied so the
		// charge is not lost, and let an operator resolve the configuration.
		service.log.Error("product on recorded license charge is no longer configured",
			zap.Stringer("user_id", record.UserID),
			zap.Int32("product_id", record.ProductID),
			zap.Stringer("record_id", record.ID),
		)
		return nil
	}

	desc := info.ProductName + licenseSeatInvoiceItemDesc
	if record.BilledDays < record.DaysInPeriod {
		desc += fmt.Sprintf(" (%d/%d days)", record.BilledDays, record.DaysInPeriod)
	}

	item := &stripe.InvoiceItemParams{
		Quantity:   stripe.Int64(record.Seats),
		UnitAmount: stripe.Int64(record.UnitAmountCents),
	}
	item.AddMetadata(LicenseSeatItemMetadataKey, LicenseSeatItemMetadataValue)
	item.AddMetadata(LicenseSeatChargeMetadataKey, licenseSeatChargeKey(record))
	if info.LicenseFeeSKU != "" && service.stripeConfig.SkuEnabled {
		item.AddMetadata("SKU", info.LicenseFeeSKU)
		item.AddMetadata("ItemCode", info.LicenseFeeSKU)
		if service.stripeConfig.InvItemSKUInDescription {
			desc += " - " + info.LicenseFeeSKU
		}
	}
	item.Description = stripe.String(desc)
	item.SetIdempotencyKey(licenseSeatChargeKey(record))

	return item
}

// InvoiceItemsFromLicenses builds the Stripe invoice items the given licenses would be
// charged for in [start, end), without recording anything. Exported for testing.
func (service *Service) InvoiceItemsFromLicenses(licenses []entitlements.AccountLicense, userID uuid.UUID, start, end time.Time) (result []*stripe.InvoiceItemParams) {
	for _, charge := range service.LicenseSeatChargesFromLicenses(licenses, userID, start, end) {
		item := service.InvoiceItemFromLicenseRecord(LicenseRecord{
			UserID:          charge.UserID,
			ProductID:       charge.ProductID,
			Seats:           charge.Seats,
			BilledDays:      charge.BilledDays,
			DaysInPeriod:    charge.DaysInPeriod,
			UnitAmountCents: charge.UnitAmountCents,
			PeriodStart:     start,
			PeriodEnd:       end,
		})
		if item == nil {
			continue
		}
		result = append(result, item)
	}

	return result
}

// sortedLicenseSeatGroups returns the groups in a deterministic order so invoice
// items are emitted consistently.
func sortedLicenseSeatGroups(seats map[licenseSeatGroup]int64) []licenseSeatGroup {
	groups := make([]licenseSeatGroup, 0, len(seats))
	for group := range seats {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].productID != groups[j].productID {
			return groups[i].productID < groups[j].productID
		}
		return groups[i].billedDays < groups[j].billedDays
	})
	return groups
}

// sortLicenseRecords orders records the same way seat groups are ordered, so invoice
// items are emitted consistently regardless of the order the database returned.
func sortLicenseRecords(records []LicenseRecord) {
	sort.Slice(records, func(i, j int) bool {
		if records[i].ProductID != records[j].ProductID {
			return records[i].ProductID < records[j].ProductID
		}
		return records[i].BilledDays < records[j].BilledDays
	})
}
