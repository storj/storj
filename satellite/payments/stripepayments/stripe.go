// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripepayments

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/satellite/payments"
)

var (
	// stripeErr is a wrapper for stripe err
	stripeErr = errs.Class("stripe error")

	mon = monkit.Package()
)

// service is payments.Service implementation which
// works with stripe network through stripe-go client
type service struct {
	log    *zap.Logger
	client *client.API
}

// NewService creates new instance of StripeService initialized with API key
func NewService(log *zap.Logger, apiKey string) payments.Service {
	stripe.DefaultLeveledLogger = log.Sugar()

	sc := new(client.API)
	sc.Init(apiKey, nil)

	return &service{
		log:    log,
		client: sc,
	}
}

// CreateCustomer creates new customer from CustomerParams struct
// sets default payment to one of the predefined testing VISA credit cards
func (s *service) CreateCustomer(ctx context.Context, params payments.CreateCustomerParams) (_ *payments.Customer, err error) {
	defer mon.Task()(&ctx)(&err)

	cparams := &stripe.CustomerParams{
		Email: stripe.String(params.Email),
		Name:  stripe.String(params.Name),
	}

	// TODO: delete after migrating from test environment
	err = cparams.SetSource("tok_visa")
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	cus, err := s.client.Customers.New(cparams)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	return &payments.Customer{
		ID:        []byte(cus.ID),
		Name:      cus.Name,
		Email:     cus.Email,
		CreatedAt: time.Unix(cus.Created, 0),
	}, nil
}

// GetCustomer retrieves customer object from stripe network
func (s *service) GetCustomer(ctx context.Context, id []byte) (_ *payments.Customer, err error) {
	defer mon.Task()(&ctx)(&err)

	cus, err := s.client.Customers.Get(string(id), nil)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	return &payments.Customer{
		ID:        []byte(cus.ID),
		Name:      cus.Name,
		Email:     cus.Email,
		CreatedAt: time.Unix(cus.Created, 0),
	}, nil
}

// AddPaymentMethod add payment method to defined customer
func (s *service) AddPaymentMethod(ctx context.Context, params payments.AddPaymentMethodParams) (*payments.PaymentMethod, error) {
	cparams := &stripe.PaymentMethodParams{
		Type: stripe.String(string(stripe.PaymentMethodTypeCard)),
		Card: &stripe.PaymentMethodCardParams{Token: &params.Token},
	}

	method, err := s.client.PaymentMethods.New(cparams)
	if err != nil {
		return nil, err
	}

	pparams := &stripe.PaymentMethodAttachParams{
		Customer: &params.CustomerID,
	}

	paymentMethod, err := s.client.PaymentMethods.Attach(method.ID, pparams)
	if err != nil {
		return nil, err
	}

	return &payments.PaymentMethod{
		CustomerID: []byte(paymentMethod.Customer.ID),
		ID:         []byte(paymentMethod.ID),
		Card: payments.Card{
			ExpYear:  int64(paymentMethod.Card.ExpYear),
			ExpMonth: int64(paymentMethod.Card.ExpMonth),
			Brand:    string(paymentMethod.Card.Brand),
			LastFour: paymentMethod.Card.Last4,
			Country:  paymentMethod.Card.Country,
		},
	}, nil
}

// GetCustomerDefaultPaymentMethod retrieves customer default payment method from stripe network
func (s *service) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID []byte) (_ *payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)

	cus, err := s.client.Customers.Get(string(customerID), nil)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	if cus.DefaultSource == nil {
		return nil, stripeErr.New("no default payment method attached to customer")
	}

	pm, err := s.client.PaymentMethods.Get(cus.DefaultSource.ID, nil)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	if pm.Type != stripe.PaymentMethodTypeCard {
		return nil, stripeErr.New("payment method other than cards are not allowed")
	}

	return &payments.PaymentMethod{
		ID:         []byte(pm.ID),
		CustomerID: []byte(cus.ID),
		Card: payments.Card{
			Country:  pm.Card.Country,
			Brand:    string(pm.Card.Brand),
			Name:     pm.BillingDetails.Name,
			ExpMonth: int64(pm.Card.ExpMonth),
			ExpYear:  int64(pm.Card.ExpYear),
			LastFour: pm.Card.Last4,
		},
		CreatedAt: time.Unix(pm.Created, 0),
	}, nil
}

// GetCustomerPaymentsMethods retrieves all payments method attached to particular customer
func (s *service) GetCustomerPaymentsMethods(ctx context.Context, customerID []byte) (_ []payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)

	pmparams := &stripe.PaymentMethodListParams{}
	pmparams.Filters.AddFilter("customer", "", string(customerID))
	pmparams.Filters.AddFilter("type", "", "card")

	iterator := s.client.PaymentMethods.List(pmparams)
	if err = iterator.Err(); err != nil {
		return nil, stripeErr.Wrap(err)
	}

	var paymentMethods []payments.PaymentMethod
	for iterator.Next() {
		pm := iterator.PaymentMethod()
		if pm.Type != stripe.PaymentMethodTypeCard {
			continue
		}

		paymentMethods = append(paymentMethods, payments.PaymentMethod{
			ID:         []byte(pm.ID),
			CustomerID: customerID,
			Card: payments.Card{
				Country:  pm.Card.Country,
				Brand:    string(pm.Card.Brand),
				Name:     pm.BillingDetails.Name,
				ExpMonth: int64(pm.Card.ExpMonth),
				ExpYear:  int64(pm.Card.ExpYear),
				LastFour: pm.Card.Last4,
			},
			CreatedAt: time.Unix(pm.Created, 0),
		})
	}

	return paymentMethods, nil
}

// GetPaymentMethod retrieve payment method object from stripe network
func (s *service) GetPaymentMethod(ctx context.Context, id []byte) (_ *payments.PaymentMethod, err error) {
	defer mon.Task()(&ctx)(&err)
	pm, err := s.client.PaymentMethods.Get(string(id), nil)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	if pm.Type != stripe.PaymentMethodTypeCard {
		return nil, stripeErr.New("payment method other than cards are not allowed")
	}

	// TODO: check if name is always returned
	var customerID []byte
	if pm.Customer != nil {
		customerID = []byte(pm.Customer.ID)
	}

	return &payments.PaymentMethod{
		ID:         []byte(pm.ID),
		CustomerID: customerID,
		Card: payments.Card{
			Country:  pm.Card.Country,
			Brand:    string(pm.Card.Brand),
			Name:     pm.BillingDetails.Name,
			ExpMonth: int64(pm.Card.ExpMonth),
			ExpYear:  int64(pm.Card.ExpYear),
			LastFour: pm.Card.Last4,
		},
		CreatedAt: time.Unix(pm.Created, 0),
	}, nil
}

// CreateProjectInvoice creates new project invoice on stripe network from input params.
// Included line items:
// - Storage
// - Egress
// - ObjectsCount
// Created invoice has AutoAdvance property set to true, so it will be finalized
// (no further editing) and attempted to be paid in 1 hour after creation
func (s *service) CreateProjectInvoice(ctx context.Context, params payments.CreateProjectInvoiceParams) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID := string(params.CustomerID)

	// create line items
	_, err = s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(customerID),
		Description: stripe.String("Storage"),
		Quantity:    stripe.Int64(int64(params.Storage)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	_, err = s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(customerID),
		Description: stripe.String("Egress"),
		Quantity:    stripe.Int64(int64(params.Egress)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	_, err = s.client.InvoiceItems.New(&stripe.InvoiceItemParams{
		Customer:    stripe.String(customerID),
		Description: stripe.String("ObjectsCount"),
		Quantity:    stripe.Int64(int64(params.ObjectCount)),
		UnitAmount:  stripe.Int64(100),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	})
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	// create invoice
	invoiceParams := &stripe.InvoiceParams{
		Customer:             stripe.String(customerID),
		DefaultPaymentMethod: stripe.String(string(params.PaymentMethodID)),
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

	inv, err := s.client.Invoices.New(invoiceParams)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	// TODO: check for more items
	var lineItems []payments.LineItem
	for _, item := range inv.Lines.Data {
		lineItems = append(lineItems, payments.LineItem{
			Key:      item.Description,
			Quantity: item.Quantity,
			Amount:   item.Amount,
		})
	}

	var customFields []payments.CustomField
	for _, field := range inv.CustomFields {
		customFields = append(customFields, payments.CustomField{
			Name:  field.Name,
			Value: field.Value,
		})
	}

	return &payments.Invoice{
		ID:           []byte(inv.ID),
		Amount:       inv.AmountDue,
		Currency:     payments.Currency(inv.Currency),
		LineItems:    lineItems,
		CustomFields: customFields,
		CreatedAt:    time.Unix(inv.Created, 0),
	}, nil
}

// GetInvoice retrieves an invoice from stripe network by invoiceID
func (s *service) GetInvoice(ctx context.Context, id []byte) (_ *payments.Invoice, err error) {
	defer mon.Task()(&ctx)(&err)
	inv, err := s.client.Invoices.Get(string(id), nil)
	if err != nil {
		return nil, stripeErr.Wrap(err)
	}

	// TODO: check for more items
	var lineItems []payments.LineItem
	for _, item := range inv.Lines.Data {
		lineItems = append(lineItems, payments.LineItem{
			Key:      item.Description,
			Quantity: item.Quantity,
			Amount:   item.Amount,
		})
	}

	var customFields []payments.CustomField
	for _, field := range inv.CustomFields {
		customFields = append(customFields, payments.CustomField{
			Name:  field.Name,
			Value: field.Value,
		})
	}

	return &payments.Invoice{
		ID:           []byte(inv.ID),
		Amount:       inv.AmountDue,
		Currency:     payments.Currency(inv.Currency),
		LineItems:    lineItems,
		CustomFields: customFields,
		CreatedAt:    time.Unix(inv.Created, 0),
	}, nil

}

// timeRangeString helper function to create string representation of time range
func timeRangeString(start, end time.Time) string {
	return fmt.Sprintf("%d/%d/%d - %d/%d/%d",
		start.UTC().Month(), start.UTC().Day(), start.UTC().Year(),
		end.UTC().Month(), end.UTC().Day(), end.UTC().Year())
}
