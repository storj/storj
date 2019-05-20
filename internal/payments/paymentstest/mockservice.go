package paymentstest

import (
	"github.com/stripe/stripe-go"

	"storj.io/storj/internal/payments"
)

// MockService is mock implementation of payments.Service for testing
type MockService struct{}

// CreateCustomer creates new *stripe.Customer with fields matching to provided CustomerParams.
// Doesn't set source and other data populated by real stripe server. Customer id is equal to
// provided email address. Error is always nil
func (mock *MockService) CreateCustomer(params payments.CustomerParams) (*stripe.Customer, error) {
	cus := &stripe.Customer{
		ID:    params.Email,
		Email: params.Email,
		Name:  params.Name,
	}

	if params.Description != "" {
		cus.Description = params.Description
	} else {
		cus.Description = params.Name
	}

	return cus, nil
}
