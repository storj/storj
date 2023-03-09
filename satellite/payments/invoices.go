// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

const (
	// InvoiceStatusDraft indicates the invoice is a draft.
	InvoiceStatusDraft = "draft"
	// InvoiceStatusOpen indicates the invoice is open.
	InvoiceStatusOpen = "open"
	// InvoiceStatusPaid indicates the invoice is paid.
	InvoiceStatusPaid = "paid"
	// InvoiceStatusUncollectible indicates the invoice is uncollectible.
	InvoiceStatusUncollectible = "uncollectible"
	// InvoiceStatusVoid indicates the invoice is void.
	InvoiceStatusVoid = "void"
)

// Invoices exposes all needed functionality to manage account invoices.
//
// architecture: Service
type Invoices interface {
	// Create creates an invoice with price and description.
	Create(ctx context.Context, userID uuid.UUID, price int64, desc string) (*Invoice, error)
	// Pay pays an invoice.
	Pay(ctx context.Context, invoiceID, paymentMethodID string) (*Invoice, error)
	// List returns a list of invoices for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]Invoice, error)
	// ListFailed returns a list of failed invoices.
	ListFailed(ctx context.Context) ([]Invoice, error)
	// ListWithDiscounts returns a list of invoices and coupon usages for a given payment account.
	ListWithDiscounts(ctx context.Context, userID uuid.UUID) ([]Invoice, []CouponUsage, error)
	// CheckPendingItems returns if pending invoice items for a given payment account exist.
	CheckPendingItems(ctx context.Context, userID uuid.UUID) (existingItems bool, err error)
	// AttemptPayOverdueInvoices attempts to pay a user's open, overdue invoices.
	AttemptPayOverdueInvoices(ctx context.Context, userID uuid.UUID) (err error)
	// Delete a draft invoice.
	Delete(ctx context.Context, id string) (inv *Invoice, err error)
}

// Invoice holds all public information about invoice.
type Invoice struct {
	ID          string    `json:"id"`
	CustomerID  string    `json:"-"`
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	Link        string    `json:"link"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}

// CouponUsage describes the usage of a coupon on an invoice.
type CouponUsage struct {
	Coupon      Coupon
	Amount      int64
	PeriodStart time.Time
	PeriodEnd   time.Time
}
