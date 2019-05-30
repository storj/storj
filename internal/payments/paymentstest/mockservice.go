// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentstest

import (
	"context"

	"github.com/stripe/stripe-go"

	"storj.io/storj/internal/payments"
)

// MockService is mock implementation of payments.Service for testing
type MockService struct{}

// CreateCustomer creates new *stripe.Customer with fields matching to provided CustomerParams.
// Doesn't set source and other data populated by real stripe server. Customer id is equal to
// provided email address. Error is always nil
func (mock *MockService) CreateCustomer(ctx context.Context, params payments.CreateCustomerParams) (*stripe.Customer, error) {
	cus := &stripe.Customer{
		ID:    params.Email,
		Email: params.Email,
		Name:  params.Name,
		DefaultSource: &stripe.PaymentSource{
			ID: "pm_id",
		},
	}

	if params.Description != "" {
		cus.Description = params.Description
	} else {
		cus.Description = params.Name
	}

	return cus, nil
}

// CreateProjectInvoice mock implementation of payments.Service CreateProjectInvoice
func (mock *MockService) CreateProjectInvoice(ctx context.Context, params payments.CreateProjectInvoiceParams) (*stripe.Invoice, error) {
	return &stripe.Invoice{ID: "invoice_id"}, nil
}

// GetInvoice mock implementation of payments.Service GetInvoice
func (mock *MockService) GetInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	return &stripe.Invoice{ID: "invoice_id", Customer: &stripe.Customer{ID: "customer_id"}}, nil
}

// GetCustomer mock implementation of payments.Service GetCustomer
func (mock *MockService) GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error) {
	return &stripe.Customer{
		ID: customerID,
		DefaultSource: &stripe.PaymentSource{
			ID: "pm_id",
		},
	}, nil
}

// GetCustomerDefaultPaymentMethod mock implementation of payments.Service GetCustomerDefaultPaymentMethod
func (mock *MockService) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID string) (*stripe.PaymentMethod, error) {
	return &stripe.PaymentMethod{ID: "pm_id", Customer: &stripe.Customer{ID: customerID}}, nil
}

// GetCustomerPaymentsMethods mock implementation of payments.Service GetCustomerPaymentsMethods
func (mock *MockService) GetCustomerPaymentsMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error) {
	return []*stripe.PaymentMethod{{ID: "pm_id", Customer: &stripe.Customer{ID: customerID}}}, nil
}

// GetPaymentMethod mock implementation of payments.Service GetPaymentMethod
func (mock *MockService) GetPaymentMethod(ctx context.Context, id string) (*stripe.PaymentMethod, error) {
	return &stripe.PaymentMethod{ID: "pm_id"}, nil
}
