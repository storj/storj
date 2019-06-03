// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package localpayments

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments"
)

// storjCreationDate is a Storj creation date. TODO: correct values
var storjCreationDate = time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)

// defaultPaymentMethod represents one and only payment method for local payments,
// which attached to all customers by default
var defaultPaymentMethod = payments.PaymentMethod{
	ID:         []byte("0"),
	CustomerID: []byte("0"),
	Card: payments.Card{
		Country:  "us",
		Brand:    "visa",
		Name:     "Storj",
		ExpMonth: 12,
		ExpYear:  2022,
		LastFour: "1488",
	},
	CreatedAt: storjCreationDate,
}

// StorjCustomer is a predefined customer
// which is linked with every user by default
var storjCustomer = payments.Customer{
	ID:        []byte("0"),
	Name:      "Storj",
	Email:     "storj@example.com",
	CreatedAt: storjCreationDate,
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

// NewService create new instance of local payments service
func NewService(db DB) payments.Service {
	return &service{db: db}
}

// CreateCustomer creates new payments.Customer with random id to satisfy unique db constraint
func (*service) CreateCustomer(ctx context.Context, params payments.CreateCustomerParams) (*payments.Customer, error) {
	var b [8]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return nil, internalPaymentsErr.New("error creating customer")
	}

	return &payments.Customer{
		ID: b[:],
	}, nil
}

// GetCustomer always returns default storjCustomer
func (*service) GetCustomer(ctx context.Context, id []byte) (*payments.Customer, error) {
	return &storjCustomer, nil
}

// GetCustomerDefaultPaymentMethod always returns defaultPaymentMethod
func (*service) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID []byte) (*payments.PaymentMethod, error) {
	return &defaultPaymentMethod, nil
}

// GetCustomerPaymentsMethods always returns payments.Customer list with defaultPaymentMethod
func (*service) GetCustomerPaymentsMethods(ctx context.Context, customerID []byte) ([]payments.PaymentMethod, error) {
	return []payments.PaymentMethod{defaultPaymentMethod}, nil
}

// GetPaymentMethod always returns defaultPaymentMethod or error
func (*service) GetPaymentMethod(ctx context.Context, id []byte) (*payments.PaymentMethod, error) {
	if string(id) == "0" {
		return &defaultPaymentMethod, nil
	}

	return nil, internalPaymentsErr.New("only one payments method exists, with id \"0\"")
}

// CreateProjectInvoice creates invoice from provided params
func (*service) CreateProjectInvoice(ctx context.Context, params payments.CreateProjectInvoiceParams) (*payments.Invoice, error) {
	// TODO: fill data
	return &payments.Invoice{}, nil
}

// GetInvoice retrieves invoice information from project invoice stamp by invoice id
// and returns invoice
func (*service) GetInvoice(ctx context.Context, id []byte) (*payments.Invoice, error) {
	// TODO: get project invoice stamp by invoice id from the db and fill data
	return &payments.Invoice{}, nil
}
