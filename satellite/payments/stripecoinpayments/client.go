// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/client"
	"github.com/stripe/stripe-go/customerbalancetransaction"
	"github.com/stripe/stripe-go/invoice"
	"github.com/stripe/stripe-go/invoiceitem"
	"github.com/stripe/stripe-go/paymentmethod"
	"go.uber.org/zap"
)

// StripeClient Stripe client interface.
type StripeClient interface {
	Customers() StripeCustomers
	PaymentMethods() StripePaymentMethods
	Invoices() StripeInvoices
	InvoiceItems() StripeInvoiceItems
	CustomerBalanceTransactions() StripeCustomerBalanceTransactions
	Charges() StripeCharges
}

// StripeCustomers Stripe Customers interface.
type StripeCustomers interface {
	New(params *stripe.CustomerParams) (*stripe.Customer, error)
	Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error)
	Update(id string, params *stripe.CustomerParams) (*stripe.Customer, error)
}

// StripePaymentMethods Stripe PaymentMethods interface.
type StripePaymentMethods interface {
	List(listParams *stripe.PaymentMethodListParams) *paymentmethod.Iter
	New(params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error)
	Attach(id string, params *stripe.PaymentMethodAttachParams) (*stripe.PaymentMethod, error)
	Detach(id string, params *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error)
}

// StripeInvoices Stripe Invoices interface.
type StripeInvoices interface {
	New(params *stripe.InvoiceParams) (*stripe.Invoice, error)
	List(listParams *stripe.InvoiceListParams) *invoice.Iter
	FinalizeInvoice(id string, params *stripe.InvoiceFinalizeParams) (*stripe.Invoice, error)
}

// StripeInvoiceItems Stripe InvoiceItems interface.
type StripeInvoiceItems interface {
	New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
	List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter
}

// StripeCharges Stripe Charges interface.
type StripeCharges interface {
	List(listParams *stripe.ChargeListParams) *charge.Iter
}

// StripeCustomerBalanceTransactions Stripe CustomerBalanceTransactions interface.
type StripeCustomerBalanceTransactions interface {
	New(params *stripe.CustomerBalanceTransactionParams) (*stripe.CustomerBalanceTransaction, error)
	List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter
}

type stripeClient struct {
	client *client.API
}

func (s *stripeClient) Customers() StripeCustomers {
	return s.client.Customers
}

func (s *stripeClient) PaymentMethods() StripePaymentMethods {
	return s.client.PaymentMethods
}

func (s *stripeClient) Invoices() StripeInvoices {
	return s.client.Invoices
}

func (s *stripeClient) InvoiceItems() StripeInvoiceItems {
	return s.client.InvoiceItems
}

func (s *stripeClient) CustomerBalanceTransactions() StripeCustomerBalanceTransactions {
	return s.client.CustomerBalanceTransactions
}

func (s *stripeClient) Charges() StripeCharges {
	return s.client.Charges
}

// NewStripeClient creates Stripe client from configuration.
func NewStripeClient(log *zap.Logger, config Config) StripeClient {
	backendConfig := &stripe.BackendConfig{
		LeveledLogger: log.Sugar(),
	}

	sClient := client.New(config.StripeSecretKey,
		&stripe.Backends{
			API:     stripe.GetBackendWithConfig(stripe.APIBackend, backendConfig),
			Connect: stripe.GetBackendWithConfig(stripe.ConnectBackend, backendConfig),
			Uploads: stripe.GetBackendWithConfig(stripe.UploadsBackend, backendConfig),
		},
	)

	return &stripeClient{client: sClient}
}
