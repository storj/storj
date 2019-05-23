// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// StripeErr is an error for stripe errors
var StripeErr = errs.Class("stripe error")

// Service is interfaces that defines behavior for working with payments
type Service interface {
	CreateCustomer(ctx context.Context, params CreateCustomerParams) (*stripe.Customer, error)
	GetCustomer(ctx context.Context, customerID string) (*stripe.Customer, error)
	GetCustomerDefaultPaymentMethod(ctx context.Context, customerID string) (*stripe.PaymentMethod, error)
	GetCustomerPaymentsMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error)
	GetPaymentMethod(ctx context.Context, id string) (*stripe.PaymentMethod, error)
	CreateProjectInvoice(ctx context.Context, params CreateProjectInvoiceParams) (*stripe.Invoice, error)
	GetInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error)
}

// StripeService works with stripe network through stripe-go client
type StripeService struct {
	log    *zap.Logger
	client *client.API
}

// CreateCustomerParams contains info needed to create new stripe customer
type CreateCustomerParams struct {
	Email       string
	Name        string
	Description string
	SourceToken string
}

// CreateProjectInvoiceParams contains info needed to create project invoice
type CreateProjectInvoiceParams struct {
	ProjectName     string
	CustomerID      string
	PaymentMethodID string

	Storage     float64
	Egress      float64
	ObjectCount float64

	StartDate time.Time
	EndDate   time.Time
}

// NewService creates new instance of StripeService initialized with API key
func NewService(log *zap.Logger, apiKey string) *StripeService {
	stripe.DefaultLeveledLogger = wrapLogger(log)

	sc := new(client.API)
	sc.Init(apiKey, nil)

	return &StripeService{
		log:    log,
		client: sc,
	}
}

// CreateCustomer creates new customer from CustomerParams struct
// sets default payment to one of the predefined testing VISA credit cards
func (s *StripeService) CreateCustomer(ctx context.Context, params CreateCustomerParams) (*stripe.Customer, error) {
	cparams := &stripe.CustomerParams{
		Email:       stripe.String(params.Email),
		Name:        stripe.String(params.Name),
		Description: stripe.String(params.Description),
	}

	// Set default source (payment instrument)
	//if params.SourceToken != "" {
	//	err := cparams.SetSource(params.SourceToken)
	//	if err != nil {
	//		return nil, StripeErr.Wrap(err)
	//	}
	//}

	// TODO: delete after migrating from test environment
	err := cparams.SetSource("tok_visa")
	if err != nil {
		return nil, err
	}

	return s.client.Customers.New(cparams)
}

// GetCustomer retrieves customer object from stripe network
func (s *StripeService) GetCustomer(ctx context.Context, id string) (*stripe.Customer, error) {
	return s.client.Customers.Get(id, nil)
}

// GetCustomerDefaultPaymentMethod retrieves customer default payment method from stripe network
func (s *StripeService) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID string) (*stripe.PaymentMethod, error) {
	cus, err := s.client.Customers.Get(customerID, nil)
	if err != nil {
		return nil, err
	}

	if cus.DefaultSource == nil {
		return nil, StripeErr.New("no default payment method attached to customer")
	}

	return s.client.PaymentMethods.Get(cus.DefaultSource.ID, nil)
}

// GetCustomerPaymentsMethods retrieves all payments method attached to particular customer
func (s *StripeService) GetCustomerPaymentsMethods(ctx context.Context, customerID string) ([]*stripe.PaymentMethod, error) {
	var err error

	pmparams := &stripe.PaymentMethodListParams{}
	pmparams.Filters.AddFilter("customer", "", customerID)
	pmparams.Filters.AddFilter("type", "", "card")

	iterator := s.client.PaymentMethods.List(pmparams)
	if err = iterator.Err(); err != nil {
		return nil, err
	}

	var paymentMethods []*stripe.PaymentMethod
	for iterator.Next() {
		pm := iterator.PaymentMethod()
		paymentMethods = append(paymentMethods, pm)
	}

	return paymentMethods, nil
}

// GetPaymentMethod retrieve payment method object from stripe network
func (s *StripeService) GetPaymentMethod(ctx context.Context, id string) (*stripe.PaymentMethod, error) {
	return s.client.PaymentMethods.Get(id, nil)
}

// CreateProjectInvoice creates new project invoice on stripe network from input params.
// Included line items:
// - Storage
// - Egress
// - ObjectsCount
// Created invoice has AutoAdvance property set to true, so it will be finalized
// (no further editing) and attempted to be paid in 1 hour after creation
func (s *StripeService) CreateProjectInvoice(ctx context.Context, params CreateProjectInvoiceParams) (*stripe.Invoice, error) {
	// create line items
	_, err := s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(params.CustomerID),
		Description: stripe.String("Storage"),
		Quantity:    stripe.Int64(int64(params.Storage)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, err
	}

	_, err = s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(params.CustomerID),
		Description: stripe.String("Egress"),
		Quantity:    stripe.Int64(int64(params.Egress)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, err
	}

	_, err = s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(params.CustomerID),
		Description: stripe.String("ObjectsCount"),
		Quantity:    stripe.Int64(int64(params.Egress)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, err
	}

	// TODO: fetch card info manually?
	// create invoice
	invoiceParams := &stripe.InvoiceParams{
		Customer:             stripe.String(params.CustomerID),
		DefaultPaymentMethod: stripe.String(params.PaymentMethodID),
		Description:          stripe.String(fmt.Sprintf("Invoice for usage of %s", params.ProjectName)),
		CustomFields: []*stripe.InvoiceCustomFieldParams{
			{
				Name:  stripe.String("Billing period"),
				Value: stripe.String(timeRangeString(params.StartDate, params.EndDate)),
			},
			{
				Name:  stripe.String("Project Name"),
				Value: stripe.String(params.ProjectName),
			},
		},
		AutoAdvance: stripe.Bool(true),
	}

	return s.client.Invoices.New(invoiceParams)
}

// GetInvoice retrieves an invoice from stripe network by invoiceID
func (s *StripeService) GetInvoice(ctx context.Context, invoiceID string) (*stripe.Invoice, error) {
	return s.client.Invoices.Get(invoiceID, nil)
}

// timeRangeString helper function to create string representation of time range
func timeRangeString(start, end time.Time) string {
	return fmt.Sprintf("%d/%d/%d - %d/%d/%d",
		start.UTC().Month(), start.UTC().Day(), start.UTC().Year(),
		end.UTC().Month(), end.UTC().Day(), end.UTC().Year())
}
