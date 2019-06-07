// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"time"
)

// Service is interfaces that defines behavior for working with payments
type Service interface {
	CreateCustomer(ctx context.Context, params CreateCustomerParams) (*Customer, error)
	GetCustomer(ctx context.Context, id []byte) (*Customer, error)
	GetCustomerDefaultPaymentMethod(ctx context.Context, customerID []byte) (*PaymentMethod, error)
	GetCustomerPaymentsMethods(ctx context.Context, customerID []byte) ([]PaymentMethod, error)
	GetPaymentMethod(ctx context.Context, id []byte) (*PaymentMethod, error)
	CreateProjectInvoice(ctx context.Context, params CreateProjectInvoiceParams) (*Invoice, error)
	GetInvoice(ctx context.Context, id []byte) (*Invoice, error)
}

// CreateCustomerParams contains info needed to create new customer
type CreateCustomerParams struct {
	Email string
	Name  string
}

// Customer contains customer info
type Customer struct {
	ID    []byte
	Name  string
	Email string

	CreatedAt time.Time
}

// Card contains credit card info
type Card struct {
	Country  string
	Brand    string
	Name     string
	ExpMonth int64
	ExpYear  int64
	LastFour string
}

// PaymentMethod contains payment method description.
// Credit cards are the only allowed payment methods so far
type PaymentMethod struct {
	ID         []byte
	CustomerID []byte

	Card Card

	CreatedAt time.Time
}

// CreateProjectInvoiceParams contains info needed to create project invoice
type CreateProjectInvoiceParams struct {
	ProjectName     string
	CustomerID      []byte
	PaymentMethodID []byte

	Storage     float64
	Egress      float64
	ObjectCount float64

	StartDate time.Time
	EndDate   time.Time
}

// Currency is type for allowed currency
type Currency string

const (
	// CurrencyUSD is USA default currency
	CurrencyUSD Currency = "usd"
)

// InvoiceStatus defines set of allowed invoice statuses
type InvoiceStatus string

const (
	// InvoiceStatusOpen describes invoice which ready to be paid
	InvoiceStatusOpen InvoiceStatus = "open"
	// InvoiceStatusPaid describes successfully paid invoice
	InvoiceStatusPaid InvoiceStatus = "paid"
	// InvoiceStatusVoid describes invoice that can be considered as a mistake
	InvoiceStatusVoid InvoiceStatus = "void"
	// InvoiceStatusUncollectible describes invoice which can be considered as debt,
	// as it's payment failed
	InvoiceStatusUncollectible InvoiceStatus = "uncollectible"
)

// ProjectLineItem defines line items for project invoice
type ProjectLineItem = string

const (
	// LineItemStorage defines line item key for storage used
	LineItemStorage ProjectLineItem = "Storage"
	// LineItemEgress defines line item key for egress used
	LineItemEgress ProjectLineItem = "Egress"
	// LineItemObjectCount defines line item key for object count
	LineItemObjectCount ProjectLineItem = "Object Count"
)

// ProjectCustomField defines custom fields for project invoice
type ProjectCustomField = string

const (
	// CustomFieldProjectName defines custom field name for project name
	CustomFieldProjectName = "Project name"
	// CustomFieldBillingPeriod defines custom field name for billing period
	CustomFieldBillingPeriod = "Billing Period"
)

// LineItem contains invoice line item info
type LineItem struct {
	Key      string
	Quantity int64
	Amount   int64
}

// CustomField represents custom field/value field
type CustomField struct {
	Name  string
	Value string
}

// Invoice holds invoice information
type Invoice struct {
	ID              []byte
	PaymentMethodID []byte

	Amount       int64
	Currency     Currency
	Status       InvoiceStatus
	LineItems    []LineItem
	CustomFields []CustomField

	CreatedAt time.Time
}
