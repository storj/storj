// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// invoices is an implementation of payments.Invoices.
//
// architecture: Service
type invoices struct {
	service *Service
}

// AttemptPayOverdueInvoices attempts to pay a user's open, overdue invoices.
func (invoices *invoices) AttemptPayOverdueInvoices(ctx context.Context, userID uuid.UUID) (err error) {
	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	params := &stripe.InvoiceListParams{
		Customer:     &customerID,
		Status:       stripe.String(string(stripe.InvoiceStatusOpen)),
		DueDateRange: &stripe.RangeQueryParams{LesserThan: time.Now().Unix()},
	}

	var errGrp errs.Group

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		params := &stripe.InvoicePayParams{}
		invResponse, err := invoices.service.stripeClient.Invoices().Pay(stripeInvoice.ID, params)
		if err != nil {
			errGrp.Add(Error.New("unable to pay invoice %s: %w", stripeInvoice.ID, err))
			continue
		}

		if invResponse != nil && invResponse.Status != stripe.InvoiceStatusPaid {
			errGrp.Add(Error.New("invoice not paid after payment triggered %s", stripeInvoice.ID))
		}

	}

	if err = invoicesIterator.Err(); err != nil {
		return Error.Wrap(err)
	}

	return errGrp.Err()
}

// List returns a list of invoices for a given payment account.
func (invoices *invoices) List(ctx context.Context, userID uuid.UUID) (invoicesList []payments.Invoice, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.InvoiceListParams{
		Customer: &customerID,
	}

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		total := stripeInvoice.Total
		for _, line := range stripeInvoice.Lines.Data {
			// If amount is negative, this is a coupon or a credit line item.
			// Add them to the total.
			if line.Amount < 0 {
				total -= line.Amount
			}
		}

		invoicesList = append(invoicesList, payments.Invoice{
			ID:          stripeInvoice.ID,
			Description: stripeInvoice.Description,
			Amount:      total,
			Status:      convertStatus(stripeInvoice.Status),
			Link:        stripeInvoice.InvoicePDF,
			Start:       time.Unix(stripeInvoice.PeriodStart, 0),
		})
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return invoicesList, nil
}

// ListWithDiscounts returns a list of invoices and coupon usages for a given payment account.
func (invoices *invoices) ListWithDiscounts(ctx context.Context, userID uuid.UUID) (invoicesList []payments.Invoice, couponUsages []payments.CouponUsage, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	params := &stripe.InvoiceListParams{
		Customer: &customerID,
	}
	params.AddExpand("data.total_discount_amounts.discount")

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		total := stripeInvoice.Total
		for _, line := range stripeInvoice.Lines.Data {
			// If amount is negative, this is a coupon or a credit line item.
			// Add them to the total.
			if line.Amount < 0 {
				total -= line.Amount
			}
		}

		invoicesList = append(invoicesList, payments.Invoice{
			ID:          stripeInvoice.ID,
			Description: stripeInvoice.Description,
			Amount:      total,
			Status:      convertStatus(stripeInvoice.Status),
			Link:        stripeInvoice.InvoicePDF,
			Start:       time.Unix(stripeInvoice.PeriodStart, 0),
		})

		for _, dcAmt := range stripeInvoice.TotalDiscountAmounts {
			if dcAmt == nil {
				return nil, nil, Error.New("discount amount is nil")
			}

			dc := dcAmt.Discount

			coupon, err := stripeDiscountToPaymentsCoupon(dc)
			if err != nil {
				return nil, nil, Error.Wrap(err)
			}

			usage := payments.CouponUsage{
				Coupon:      *coupon,
				Amount:      dcAmt.Amount,
				PeriodStart: time.Unix(stripeInvoice.PeriodStart, 0),
				PeriodEnd:   time.Unix(stripeInvoice.PeriodEnd, 0),
			}

			if dc.PromotionCode != nil {
				usage.Coupon.PromoCode = dc.PromotionCode.Code
			}

			couponUsages = append(couponUsages, usage)
		}
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return invoicesList, couponUsages, nil
}

// CheckPendingItems returns if pending invoice items for a given payment account exist.
func (invoices *invoices) CheckPendingItems(ctx context.Context, userID uuid.UUID) (existingItems bool, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return false, Error.Wrap(err)
	}

	params := &stripe.InvoiceItemListParams{
		Customer: &customerID,
		Pending:  stripe.Bool(true),
	}

	itemIterator := invoices.service.stripeClient.InvoiceItems().List(params)
	for itemIterator.Next() {
		item := itemIterator.InvoiceItem()
		if item != nil {
			return true, nil
		}
	}

	if err = itemIterator.Err(); err != nil {
		return false, Error.Wrap(err)
	}

	return false, nil
}

// Delete a draft invoice.
func (invoices *invoices) Delete(ctx context.Context, id string) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	stripeInvoice, err := invoices.service.stripeClient.Invoices().Del(id, &stripe.InvoiceParams{})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &payments.Invoice{
		ID:          stripeInvoice.ID,
		Description: stripeInvoice.Description,
		Amount:      stripeInvoice.AmountDue,
		Status:      convertStatus(stripeInvoice.Status),
		Link:        stripeInvoice.InvoicePDF,
		Start:       time.Unix(stripeInvoice.PeriodStart, 0),
	}, nil
}

func convertStatus(stripestatus stripe.InvoiceStatus) string {
	var status string
	switch stripestatus {
	case stripe.InvoiceStatusDraft:
		status = payments.InvoiceStatusDraft
	case stripe.InvoiceStatusOpen:
		status = payments.InvoiceStatusOpen
	case stripe.InvoiceStatusPaid:
		status = payments.InvoiceStatusPaid
	case stripe.InvoiceStatusUncollectible:
		status = payments.InvoiceStatusUncollectible
	case stripe.InvoiceStatusVoid:
		status = payments.InvoiceStatusVoid
	default:
		status = string(stripestatus)
	}
	return status
}
