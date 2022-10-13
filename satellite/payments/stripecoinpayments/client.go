// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"github.com/stripe/stripe-go/v72/client"
	"github.com/stripe/stripe-go/v72/customerbalancetransaction"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/invoiceitem"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/promotioncode"
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
	PromoCodes() StripePromoCodes
	CreditNotes() StripeCreditNotes
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
	Update(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error)
	FinalizeInvoice(id string, params *stripe.InvoiceFinalizeParams) (*stripe.Invoice, error)
	Pay(id string, params *stripe.InvoicePayParams) (*stripe.Invoice, error)
}

// StripeInvoiceItems Stripe InvoiceItems interface.
type StripeInvoiceItems interface {
	New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
	Update(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
	List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter
	Del(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
}

// StripeCharges Stripe Charges interface.
type StripeCharges interface {
	List(listParams *stripe.ChargeListParams) *charge.Iter
}

// StripePromoCodes is the Stripe PromoCodes interface.
type StripePromoCodes interface {
	List(params *stripe.PromotionCodeListParams) *promotioncode.Iter
}

// StripeCustomerBalanceTransactions Stripe CustomerBalanceTransactions interface.
type StripeCustomerBalanceTransactions interface {
	New(params *stripe.CustomerBalanceTransactionParams) (*stripe.CustomerBalanceTransaction, error)
	List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter
}

// StripeCreditNotes Stripe CreditNotes interface.
type StripeCreditNotes interface {
	New(params *stripe.CreditNoteParams) (*stripe.CreditNote, error)
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

func (s *stripeClient) PromoCodes() StripePromoCodes {
	return s.client.PromotionCodes
}

func (s *stripeClient) CreditNotes() StripeCreditNotes {
	return s.client.CreditNotes
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
