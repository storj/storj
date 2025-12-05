// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

// invoices is an implementation of payments.Invoices.
//
// architecture: Service
type invoices struct {
	service *Service
}

func (invoices *invoices) Create(ctx context.Context, userID uuid.UUID, price int64, desc string) (*payments.Invoice, error) {
	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	inv, err := invoices.service.stripeClient.Invoices().New(&stripe.InvoiceParams{
		Params:                      stripe.Params{Context: ctx},
		Customer:                    stripe.String(customerID),
		Discounts:                   []*stripe.InvoiceDiscountParams{},
		Description:                 stripe.String(desc),
		PendingInvoiceItemsBehavior: stripe.String("exclude"),
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	item, err := invoices.service.stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
		Params:      stripe.Params{Context: ctx},
		Customer:    stripe.String(customerID),
		Amount:      stripe.Int64(price),
		Description: stripe.String(desc),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
		Invoice:     stripe.String(inv.ID),
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &payments.Invoice{
		ID:          inv.ID,
		Description: inv.Description,
		Amount:      item.Amount,
		Status:      string(inv.Status),
	}, nil
}

func (invoices *invoices) Pay(ctx context.Context, invoiceID, paymentMethodID string) (*payments.Invoice, error) {
	inv, err := invoices.service.stripeClient.Invoices().Pay(invoiceID, &stripe.InvoicePayParams{
		Params:        stripe.Params{Context: ctx},
		PaymentMethod: stripe.String(paymentMethodID),
	})
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return nil, Error.Wrap(err)
	}
	return &payments.Invoice{
		ID:          inv.ID,
		Description: inv.Description,
		Amount:      inv.AmountPaid,
		Status:      string(inv.Status),
	}, nil
}

func (invoices *invoices) Get(ctx context.Context, invoiceID string) (*payments.Invoice, error) {
	params := &stripe.InvoiceParams{
		Params: stripe.Params{
			Context: ctx,
		},
	}
	inv, err := invoices.service.stripeClient.Invoices().Get(invoiceID, params)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	total := inv.Total
	if inv.Lines != nil {
		for _, line := range inv.Lines.Data {
			// If amount is negative, this is a coupon or a credit line item.
			// Add them to the total.
			if line.Amount < 0 {
				total -= line.Amount
			}
		}
	}

	return &payments.Invoice{
		ID:          inv.ID,
		CustomerID:  inv.Customer.ID,
		Description: inv.Description,
		Amount:      total,
		Status:      convertStatus(inv.Status),
		Link:        inv.InvoicePDF,
		Start:       time.Unix(inv.PeriodStart, 0),
	}, nil
}

// AttemptPayOverdueInvoices attempts to pay a user's open, overdue invoices.
func (invoices *invoices) AttemptPayOverdueInvoices(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	stripeInvoices, err := invoices.service.getInvoices(ctx, customerID, time.Unix(0, 0))
	if err != nil {
		return Error.Wrap(err)
	}

	if len(stripeInvoices) == 0 {
		return nil
	}

	// first check users token balance
	monetaryTokenBalance, err := invoices.service.billingDB.GetBalance(ctx, userID)
	if err != nil {
		invoices.service.log.Error("error getting token balance", zap.Error(err))
		return Error.Wrap(err)
	}
	if monetaryTokenBalance.BaseUnits() > 0 {
		err := invoices.service.PayInvoicesWithTokenBalance(ctx, userID, customerID, stripeInvoices)
		if err != nil {
			invoices.service.log.Error("error paying invoice(s) with token balance", zap.Error(err))
			return Error.Wrap(err)
		}
		// get invoices again to see if any are still unpaid
		stripeInvoices, err = invoices.service.getInvoices(ctx, customerID, time.Unix(0, 0))
		if err != nil {
			invoices.service.log.Error("error getting invoices for stripe customer", zap.String(customerID, customerID), zap.Error(err))
			return Error.Wrap(err)
		}
	}
	if len(stripeInvoices) > 0 {
		return invoices.attemptPayOverdueInvoicesWithCC(ctx, stripeInvoices)
	}
	return nil
}

// AttemptPayOverdueInvoicesWithTokens attempts to pay a user's open, overdue invoices with tokens only.
func (invoices *invoices) AttemptPayOverdueInvoicesWithTokens(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	stripeInvoices, err := invoices.service.getInvoices(ctx, customerID, time.Unix(0, 0))
	if err != nil {
		return Error.Wrap(err)
	}

	if len(stripeInvoices) == 0 {
		return nil
	}

	// first check users token balance
	monetaryTokenBalance, err := invoices.service.billingDB.GetBalance(ctx, userID)
	if err != nil {
		invoices.service.log.Error("error getting token balance", zap.Error(err))
		return Error.Wrap(err)
	}
	if monetaryTokenBalance.BaseUnits() == 0 {
		return Error.New("User has no tokens")
	}
	err = invoices.service.PayInvoicesWithTokenBalance(ctx, userID, customerID, stripeInvoices)
	if err != nil {
		invoices.service.log.Error("error paying invoice(s) with token balance", zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}

// AttemptPayOverdueInvoices attempts to pay a user's open, overdue invoices.
func (invoices *invoices) attemptPayOverdueInvoicesWithCC(ctx context.Context, stripeInvoices []stripe.Invoice) (err error) {
	var errGrp errs.Group

	for _, stripeInvoice := range stripeInvoices {
		params := &stripe.InvoicePayParams{Params: stripe.Params{Context: ctx}}
		invResponse, err := invoices.service.stripeClient.Invoices().Pay(stripeInvoice.ID, params)
		if err != nil {
			errGrp.Add(Error.New("unable to pay invoice %s: %w", stripeInvoice.ID, err))
			continue
		}

		if invResponse != nil && invResponse.Status != stripe.InvoiceStatusPaid {
			errGrp.Add(Error.New("invoice not paid after payment triggered %s", stripeInvoice.ID))
		}

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
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
	}

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		total := stripeInvoice.Total
		if stripeInvoice.Lines != nil {
			for _, line := range stripeInvoice.Lines.Data {
				// If amount is negative, this is a coupon or a credit line item.
				// Add them to the total.
				if line.Amount < 0 {
					total -= line.Amount
				}
			}
		}

		invoicesList = append(invoicesList, payments.Invoice{
			ID:          stripeInvoice.ID,
			CustomerID:  customerID,
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

func (invoices *invoices) ListFailed(ctx context.Context, userID *uuid.UUID) (invoicesList []payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Status:     stripe.String(string(stripe.InvoiceStatusOpen)),
	}

	if userID != nil {
		customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, *userID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		params.Customer = &customerID
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

		if invoices.isInvoiceFailed(stripeInvoice) {
			invoicesList = append(invoicesList, payments.Invoice{
				ID:          stripeInvoice.ID,
				CustomerID:  stripeInvoice.Customer.ID,
				Description: stripeInvoice.Description,
				Amount:      total,
				Status:      string(stripeInvoice.Status),
				Link:        stripeInvoice.InvoicePDF,
				Start:       time.Unix(stripeInvoice.PeriodStart, 0),
			})
		}
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return invoicesList, nil
}

func (invoices *invoices) ListPaged(ctx context.Context, userID uuid.UUID, cursor payments.InvoiceCursor) (page *payments.InvoicePage, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	page = &payments.InvoicePage{}
	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
	}

	isNext := cursor.StartingAfter != ""
	isPrevious := cursor.EndingBefore != ""

	// stripe will initially fetch this number of invoices.
	// Calling iter.Next() at the end will fetch another batch
	// if there's more.
	params.Limit = stripe.Int64(int64(cursor.Limit))
	if isNext {
		page.Previous = true
		params.StartingAfter = stripe.String(cursor.StartingAfter)
	} else if isPrevious {
		page.Next = true
		params.EndingBefore = stripe.String(cursor.EndingBefore)
	}

	invoicesIterator := invoices.service.stripeClient.Invoices().List(params)
	for invoicesIterator.Next() {
		stripeInvoice := invoicesIterator.Invoice()

		if stripeInvoice.Status != stripe.InvoiceStatusOpen && stripeInvoice.Status != stripe.InvoiceStatusPaid {
			continue
		}

		if len(page.Invoices) == cursor.Limit {
			if isPrevious {
				page.Previous = true
			} else {
				page.Next = true
			}
			break
		}

		total := stripeInvoice.Total
		for _, line := range stripeInvoice.Lines.Data {
			// If amount is negative, this is a coupon or a credit line item.
			// Add them to the total.
			if line.Amount < 0 {
				total -= line.Amount
			}
		}

		var start, end time.Time
		if len(stripeInvoice.Lines.Data) > 0 {
			// For an invoice created via the Stripe API, the period on the invoice itself
			// is the date the invoice was created. See https://docs.stripe.com/stripe-data/query-billing-data#working-with-invoice-dates-and-periods
			// So we take the period from the first line item instead.
			line := stripeInvoice.Lines.Data[0]
			if line.Period != nil {
				start = time.Unix(line.Period.Start, 0)
				end = time.Unix(line.Period.End, 0)
			} else {
				start = time.Unix(stripeInvoice.PeriodStart, 0)
				end = time.Unix(stripeInvoice.PeriodEnd, 0)
			}
		}

		page.Invoices = append(page.Invoices, payments.Invoice{
			ID:          stripeInvoice.ID,
			CustomerID:  stripeInvoice.Customer.ID,
			Description: stripeInvoice.Description,
			Amount:      total,
			Status:      string(stripeInvoice.Status),
			Link:        stripeInvoice.InvoicePDF,
			Start:       start,
			End:         end,
		})
	}

	if err = invoicesIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	// Reverse the slice if we are fetching the previous page.
	// Providing cursor.EndingBefore makes iterator to return items in reversed order.
	if isPrevious {
		for i, j := 0, len(page.Invoices)-1; i < j; i, j = i+1, j-1 {
			page.Invoices[i], page.Invoices[j] = page.Invoices[j], page.Invoices[i]
		}
	}

	return page, nil
}

// ListWithDiscounts returns a list of invoices and coupon usages for a given payment account.
func (invoices *invoices) ListWithDiscounts(ctx context.Context, userID uuid.UUID) (invoicesList []payments.Invoice, couponUsages []payments.CouponUsage, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := invoices.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	params := &stripe.InvoiceListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
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
			CustomerID:  customerID,
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
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Pending:    stripe.Bool(true),
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

	params := &stripe.InvoiceParams{Params: stripe.Params{Context: ctx}}
	stripeInvoice, err := invoices.service.stripeClient.Invoices().Del(id, params)
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

// isInvoiceFailed returns whether an invoice has failed.
func (invoices *invoices) isInvoiceFailed(invoice *stripe.Invoice) bool {
	if invoice.Status != stripe.InvoiceStatusOpen || !invoice.Attempted {
		return false
	}

	if invoice.DueDate > 0 {
		// https://github.com/storj/storj/blob/77bf88e916a10dc898ebb594eafac667ed4426cd/satellite/payments/stripecoinpayments/service.go#L781-L787
		invoices.service.log.Info("Skipping invoice marked for manual payment",
			zap.String("id", invoice.ID),
			zap.String("number", invoice.Number),
			zap.String("customer", invoice.Customer.ID))
		return false
	}
	// https://stripe.com/docs/api/invoices/retrieve
	if invoice.NextPaymentAttempt > 0 {
		// stripe will automatically retry collecting payment.
		return false
	}

	return true
}
