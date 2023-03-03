// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"bytes"
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"github.com/stripe/stripe-go/v72/client"
	"github.com/stripe/stripe-go/v72/customerbalancetransaction"
	"github.com/stripe/stripe-go/v72/form"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/invoiceitem"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/promotioncode"
	"go.uber.org/zap"

	"storj.io/common/time2"
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
	Del(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error)
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
	sClient := client.New(config.StripeSecretKey,
		&stripe.Backends{
			API:     NewBackendWrapper(log, stripe.APIBackend, config.Retries),
			Connect: NewBackendWrapper(log, stripe.ConnectBackend, config.Retries),
			Uploads: NewBackendWrapper(log, stripe.UploadsBackend, config.Retries),
		},
	)

	return &stripeClient{client: sClient}
}

// RetryConfig contains the configuration for an exponential backoff strategy when retrying Stripe API calls.
type RetryConfig struct {
	InitialBackoff time.Duration `help:"the duration of the first retry interval" default:"20ms"`
	MaxBackoff     time.Duration `help:"the maximum duration of any retry interval" default:"5s"`
	Multiplier     float64       `help:"the factor by which the retry interval will be multiplied on each iteration" default:"2"`
	MaxRetries     int64         `help:"the maximum number of times to retry a request" default:"10"`
}

// BackendWrapper is a wrapper for the Stripe backend that uses an exponential backoff strategy for retrying Stripe API calls.
type BackendWrapper struct {
	backend  stripe.Backend
	retryCfg RetryConfig
	clock    time2.Clock
}

// NewBackendWrapper creates a new wrapper for a Stripe backend.
func NewBackendWrapper(log *zap.Logger, backendType stripe.SupportedBackend, retryCfg RetryConfig) *BackendWrapper {
	backendConfig := &stripe.BackendConfig{
		LeveledLogger: log.Sugar(),
		// Disable internal retries since we have our own retry+backoff strategy.
		MaxNetworkRetries: stripe.Int64(0),
	}

	return &BackendWrapper{
		retryCfg: retryCfg,
		backend:  stripe.GetBackendWithConfig(backendType, backendConfig),
	}
}

// TestSwapBackend replaces the wrapped backend with the one specified for use in testing.
func (w *BackendWrapper) TestSwapBackend(backend stripe.Backend) {
	w.backend = backend
}

// TestSwapClock replaces the internal clock with the one specified for use in testing.
func (w *BackendWrapper) TestSwapClock(clock time2.Clock) {
	w.clock = clock
}

// Call implements the stripe.Backend interface.
func (w *BackendWrapper) Call(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	return w.withRetries(params, func() error {
		return w.backend.Call(method, path, key, params, v)
	})
}

// CallStreaming implements the stripe.Backend interface.
func (w *BackendWrapper) CallStreaming(method, path, key string, params stripe.ParamsContainer, v stripe.StreamingLastResponseSetter) error {
	return w.withRetries(params, func() error {
		return w.backend.CallStreaming(method, path, key, params, v)
	})
}

// CallRaw implements the stripe.Backend interface.
func (w *BackendWrapper) CallRaw(method, path, key string, body *form.Values, params *stripe.Params, v stripe.LastResponseSetter) error {
	return w.withRetries(params, func() error {
		return w.backend.CallRaw(method, path, key, body, params, v)
	})
}

// CallMultipart implements the stripe.Backend interface.
func (w *BackendWrapper) CallMultipart(method, path, key, boundary string, body *bytes.Buffer, params *stripe.Params, v stripe.LastResponseSetter) error {
	return w.withRetries(params, func() error {
		return w.backend.CallMultipart(method, path, key, boundary, body, params, v)
	})
}

// SetMaxNetworkRetries sets the maximum number of times to retry failed requests.
func (w *BackendWrapper) SetMaxNetworkRetries(max int64) {
	w.retryCfg.MaxRetries = max
}

// withRetries executes the provided Stripe API call using an exponential backoff strategy
// for retrying in the case of failure.
func (w *BackendWrapper) withRetries(params stripe.ParamsContainer, call func() error) error {
	ctx := context.Background()
	if params != nil {
		innerParams := params.GetParams()
		if innerParams != nil && innerParams.Context != nil {
			ctx = innerParams.Context
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	backoff := float64(w.retryCfg.InitialBackoff)
	for retry := int64(0); ; retry++ {
		err := call()
		if err == nil {
			return nil
		}

		if !w.shouldRetry(retry, err) {
			return err
		}

		if !w.clock.Sleep(ctx, time.Duration(backoff)) {
			return ctx.Err()
		}

		backoff = math.Min(backoff*w.retryCfg.Multiplier, float64(w.retryCfg.MaxBackoff))
	}
}

// shouldRetry returns whether a Stripe API call should be retried.
func (w *BackendWrapper) shouldRetry(retry int64, err error) bool {
	if retry >= w.retryCfg.MaxRetries {
		return false
	}

	var stripeErr *stripe.Error
	if !errors.As(err, &stripeErr) {
		return false
	}

	resp := stripeErr.LastResponse
	if resp == nil {
		return false
	}

	switch resp.Header.Get("Stripe-Should-Retry") {
	case "true":
		return true
	case "false":
		return false
	}

	return resp.StatusCode == http.StatusTooManyRequests
}
