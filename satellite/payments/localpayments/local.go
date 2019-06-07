// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package localpayments

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
)

var (
	// creationDate is a Storj creation date.
	creationDate = time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)

	mon = monkit.Package()
)

// storjCustomer is a predefined customer
// which is linked with every user by default
var storjCustomer = payments.Customer{
	ID:        []byte("0"),
	Name:      "Storj",
	Email:     "storj@example.com",
	CreatedAt: creationDate,
}

// defaultPaymentMethod represents one and only payment method for local payments,
// which is attached to all customers by default
var defaultPaymentMethod = payments.PaymentMethod{
	ID:         []byte("0"),
	CustomerID: []byte("0"),
	Card: payments.Card{
		Country:  "us",
		Brand:    "visa",
		Name:     "Storj Labs",
		ExpMonth: 12,
		ExpYear:  2024,
		LastFour: "3567",
	},
	CreatedAt: creationDate,
}

// localPaymentsErr is a wrapper for local payments service errors
var localPaymentsErr = errs.Class("internal payments error")

// DB is internal payment methods storage
type DB interface {
	CreateInvoice(ctx context.Context, invoice payments.Invoice) (*payments.Invoice, error)
	GetInvoice(ctx context.Context, id []byte) (*payments.Invoice, error)
}

// service is internal payments.Service implementation
type service struct {
	log *zap.Logger
	db  DB
}

// NewService create new instance of local payments service
func NewService(log *zap.Logger, db DB) payments.Service {
	return &service{log: log, db: db}
}

// CreateCustomer creates new payments.Customer with random id to satisfy unique db constraint
func (*service) CreateCustomer(ctx context.Context, params payments.CreateCustomerParams) (_ *payments.Customer, err error) {
	defer mon.Task()(&ctx)(&err)

	var b [8]byte

	_, err = rand.Read(b[:])
	if err != nil {
		return nil, localPaymentsErr.New("error creating customer")
	}

	return &payments.Customer{
		ID: b[:],
	}, nil
}

// GetCustomer always returns default storjCustomer
func (*service) GetCustomer(ctx context.Context, id []byte) (_ *payments.Customer, err error) {
	defer mon.Task()(&ctx)(&err)
	return &storjCustomer, nil
}

// GetCustomerDefaultPaymentMethod always returns defaultPaymentMethod
func (*service) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID []byte) (_ *payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	return &defaultPaymentMethod, nil
}

// GetCustomerPaymentsMethods always returns payments.Customer list with defaultPaymentMethod
func (*service) GetCustomerPaymentsMethods(ctx context.Context, customerID []byte) (_ []payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	return []payments.PaymentMethod{defaultPaymentMethod}, nil
}

// GetPaymentMethod returns defaultPaymentMethod or error otherwise
func (*service) GetPaymentMethod(ctx context.Context, id []byte) (_ *payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	if string(id) == "0" {
		return &defaultPaymentMethod, nil
	}

	return nil, localPaymentsErr.New("only one payment method exists, with id \"0\"")
}

// CreateProjectInvoice creates invoice from provided params.
// Returned invoice receives InvoiceStatusPaid by default
func (s *service) CreateProjectInvoice(ctx context.Context, params payments.CreateProjectInvoiceParams) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	var amount int64
	var lineItems []payments.LineItem
	var customFields []payments.CustomField

	// line items
	storage := payments.LineItem{
		Key:      payments.LineItemStorage,
		Quantity: int64(params.Storage),
		Amount:   int64(params.Storage),
	}
	egress := payments.LineItem{
		Key:      payments.LineItemEgress,
		Quantity: int64(params.Egress),
		Amount:   int64(params.Egress),
	}
	objectCount := payments.LineItem{
		Key:      payments.LineItemObjectCount,
		Quantity: int64(params.ObjectCount),
		Amount:   int64(params.ObjectCount),
	}

	// custom fields
	billingPeriod := payments.CustomField{
		Name:  payments.CustomFieldBillingPeriod,
		Value: fmt.Sprintf("%s - %s", params.StartDate, params.EndDate),
	}
	projectName := payments.CustomField{
		Name:  payments.CustomFieldProjectName,
		Value: params.ProjectName,
	}

	lineItems = append(lineItems, storage, egress, objectCount)
	customFields = append(customFields, billingPeriod, projectName)

	amount = storage.Amount
	amount += egress.Amount
	amount += objectCount.Amount

	inv, err := s.db.CreateInvoice(ctx,
		payments.Invoice{
			PaymentMethodID: params.PaymentMethodID,
			Amount:          amount,
			Currency:        payments.CurrencyUSD,
			Status:          payments.InvoiceStatusPaid,
			LineItems:       lineItems,
			CustomFields:    customFields,
		},
	)

	return inv, localPaymentsErr.Wrap(err)
}

// GetInvoice retrieves invoice information from project invoice stamp by invoice id
// and returns invoice
func (s *service) GetInvoice(ctx context.Context, id []byte) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	inv, err := s.db.GetInvoice(ctx, id)
	return inv, localPaymentsErr.Wrap(err)
}
