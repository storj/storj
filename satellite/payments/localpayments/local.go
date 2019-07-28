// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package localpayments

import (
	"context"
	"crypto/rand"
	mathRand "math/rand"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
)

var (
	// creationDate is a Storj creation date.
	creationDate = time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)

	mon = monkit.Package()
)

// StorjCustomer is a predefined customer
// which is linked with every user by default
var storjCustomer = payments.Customer{
	ID:        []byte("0"),
	Name:      "Storj",
	Email:     "storj@mail.test",
	CreatedAt: creationDate,
}

// internalPaymentsErr is a wrapper for local payments service errors
var internalPaymentsErr = errs.Class("internal payments error")

// DB is internal payment methods storage
type DB interface {
	// TODO: add method to retrieve invoice information from project invoice stamp
}

// service is internal payments.Service implementation
type service struct {
	db DB
}

func (*service) AddPaymentMethod(ctx context.Context, params payments.AddPaymentMethodParams) (*payments.PaymentMethod, error) {
	return paymentMethod("", []byte(params.CustomerID)), nil
}

// NewService create new instance of local payments service
func NewService(db DB) payments.Service {
	return &service{db: db}
}

// CreateCustomer creates new payments.Customer with random id to satisfy unique db constraint
func (*service) CreateCustomer(ctx context.Context, params payments.CreateCustomerParams) (_ *payments.Customer, err error) {
	defer mon.Task()(&ctx)(&err)

	var b [8]byte

	_, err = rand.Read(b[:])
	if err != nil {
		return nil, internalPaymentsErr.New("error creating customer")
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
	return paymentMethod("", customerID), nil
}

// GetCustomerPaymentsMethods always returns payments.Customer list with defaultPaymentMethod
func (*service) GetCustomerPaymentsMethods(ctx context.Context, customerID []byte) (_ []payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	return []payments.PaymentMethod{*paymentMethod("", customerID)}, nil
}

// GetPaymentMethod always returns defaultPaymentMethod or error
func (*service) GetPaymentMethod(ctx context.Context, id []byte) (_ *payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	return paymentMethod(string(id), []byte("")), nil
}

// CreateProjectInvoice creates invoice from provided params
func (*service) CreateProjectInvoice(ctx context.Context, params payments.CreateProjectInvoiceParams) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: fill data
	return &payments.Invoice{}, nil
}

// GetInvoice retrieves invoice information from project invoice stamp by invoice id
// and returns invoice
func (*service) GetInvoice(ctx context.Context, id []byte) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: get project invoice stamp by invoice id from the db and fill data
	return &payments.Invoice{}, nil
}

// paymentMethod returns paymentMethod object which mocks stripe response
func paymentMethod(methodID string, customerID []byte) *payments.PaymentMethod {
	id := methodID
	if methodID == "" {
		id = "pm_" + randomString(24)
	}

	cusID := customerID
	if len(customerID) <= 1 {
		cusID = []byte("cus_" + randomString(14))
	}

	return &payments.PaymentMethod{
		ID:         []byte(id),
		CustomerID: cusID,
		Card: payments.Card{
			Country:  "us",
			Brand:    "visa",
			Name:     "Storj Labs",
			ExpMonth: 12,
			ExpYear:  2024,
			LastFour: "3567",
		},
		CreatedAt: creationDate,
		IsDefault: true,
	}
}

func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(65 + mathRand.Intn(25))
	}
	return string(bytes)
}
