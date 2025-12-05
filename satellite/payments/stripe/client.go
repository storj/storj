// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/charge"
	"github.com/stripe/stripe-go/v81/creditnote"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/customerbalancetransaction"
	"github.com/stripe/stripe-go/v81/form"
	"github.com/stripe/stripe-go/v81/invoice"
	"github.com/stripe/stripe-go/v81/invoiceitem"
	"github.com/stripe/stripe-go/v81/paymentintent"
	"github.com/stripe/stripe-go/v81/paymentmethod"
	"github.com/stripe/stripe-go/v81/promotioncode"
	"github.com/stripe/stripe-go/v81/setupintent"
	"github.com/stripe/stripe-go/v81/taxid"
	"go.uber.org/zap"

	"storj.io/common/time2"
)

// Client Stripe client interface.
type Client interface {
	Customers() Customers
	PaymentMethods() PaymentMethods
	PaymentIntents() PaymentIntents
	SetupIntents() SetupIntents
	Invoices() Invoices
	InvoiceItems() InvoiceItems
	CustomerBalanceTransactions() CustomerBalanceTransactions
	Charges() Charges
	PromoCodes() PromoCodes
	CreditNotes() CreditNotes
	TaxIDs() TaxIDs
}

// Customers Stripe Customers interface.
type Customers interface {
	New(params *stripe.CustomerParams) (*stripe.Customer, error)
	Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error)
	Update(id string, params *stripe.CustomerParams) (*stripe.Customer, error)
	List(listParams *stripe.CustomerListParams) *customer.Iter
}

// PaymentMethods Stripe PaymentMethods interface.
type PaymentMethods interface {
	List(listParams *stripe.PaymentMethodListParams) *paymentmethod.Iter
	New(params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error)
	Update(id string, params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error)
	Get(id string, params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error)
	Attach(id string, params *stripe.PaymentMethodAttachParams) (*stripe.PaymentMethod, error)
	Detach(id string, params *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error)
}

// PaymentIntents Stripe PaymentIntents interface.
type PaymentIntents interface {
	New(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error)
}

// SetupIntents Stripe SetupIntents interface.
type SetupIntents interface {
	New(params *stripe.SetupIntentParams) (*stripe.SetupIntent, error)
}

// Invoices Stripe Invoices interface.
type Invoices interface {
	New(params *stripe.InvoiceParams) (*stripe.Invoice, error)
	List(listParams *stripe.InvoiceListParams) *invoice.Iter
	Update(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error)
	FinalizeInvoice(id string, params *stripe.InvoiceFinalizeInvoiceParams) (*stripe.Invoice, error)
	Pay(id string, params *stripe.InvoicePayParams) (*stripe.Invoice, error)
	Del(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error)
	Get(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error)
	MarkUncollectible(id string, params *stripe.InvoiceMarkUncollectibleParams) (*stripe.Invoice, error)
	VoidInvoice(id string, params *stripe.InvoiceVoidInvoiceParams) (*stripe.Invoice, error)
}

// InvoiceItems Stripe InvoiceItems interface.
type InvoiceItems interface {
	New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
	Update(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
	List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter
	Del(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error)
}

// Charges Stripe Charges interface.
type Charges interface {
	List(listParams *stripe.ChargeListParams) *charge.Iter
}

// PromoCodes is the Stripe PromoCodes interface.
type PromoCodes interface {
	List(params *stripe.PromotionCodeListParams) *promotioncode.Iter
}

// TaxIDs is the Stripe TaxIDs interface.
type TaxIDs interface {
	New(params *stripe.TaxIDParams) (*stripe.TaxID, error)
	Del(id string, params *stripe.TaxIDParams) (*stripe.TaxID, error)
}

// CustomerBalanceTransactions Stripe CustomerBalanceTransactions interface.
type CustomerBalanceTransactions interface {
	New(params *stripe.CustomerBalanceTransactionParams) (*stripe.CustomerBalanceTransaction, error)
	List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter
}

// CreditNotes Stripe CreditNotes interface.
type CreditNotes interface {
	New(params *stripe.CreditNoteParams) (*stripe.CreditNote, error)
}

type stripeClient struct {
	// charges is the client used to invoke /charges APIs.
	charges *charge.Client
	// creditNotes is the client used to invoke /credit_notes APIs.
	creditNotes *creditnote.Client
	// customerBalanceTransactions is the client used to invoke /customers/{customer}/balance_transactions APIs.
	customerBalanceTransactions *customerbalancetransaction.Client
	// customers is the client used to invoke /customers APIs.
	customers *customer.Client
	// invoiceItems is the client used to invoke /invoiceitems APIs.
	invoiceItems *invoiceitem.Client
	// invoices is the client used to invoke /invoices APIs.
	invoices *invoice.Client
	// paymentMethods is the client used to invoke /payment_methods APIs.
	paymentMethods *paymentmethod.Client
	// paymentIntents is the client used to invoke /payment_intents APIs.
	paymentIntents *paymentintent.Client
	// setupIntents is the client used to invoke /setup_intents APIs.
	setupIntents *setupintent.Client
	// promotionCodes is the client used to invoke /promotion_codes APIs.
	promotionCodes *promotioncode.Client
	// taxIDs is the client used to invoke /tax_ids APIs.
	taxIDs *taxid.Client
}

func (s *stripeClient) Charges() Charges         { return s.charges }
func (s *stripeClient) CreditNotes() CreditNotes { return s.creditNotes }
func (s *stripeClient) CustomerBalanceTransactions() CustomerBalanceTransactions {
	return s.customerBalanceTransactions
}
func (s *stripeClient) Customers() Customers           { return s.customers }
func (s *stripeClient) InvoiceItems() InvoiceItems     { return s.invoiceItems }
func (s *stripeClient) Invoices() Invoices             { return s.invoices }
func (s *stripeClient) PaymentMethods() PaymentMethods { return s.paymentMethods }
func (s *stripeClient) PaymentIntents() PaymentIntents { return s.paymentIntents }
func (s *stripeClient) SetupIntents() SetupIntents     { return s.setupIntents }
func (s *stripeClient) PromoCodes() PromoCodes         { return s.promotionCodes }
func (s *stripeClient) TaxIDs() TaxIDs                 { return s.taxIDs }

// NewStripeClient creates Stripe client from configuration.
func NewStripeClient(log *zap.Logger, config Config) Client {
	key := config.StripeSecretKey
	backends := &stripe.Backends{
		API:     NewBackendWrapper(log, stripe.APIBackend, config.Retries),
		Connect: NewBackendWrapper(log, stripe.ConnectBackend, config.Retries),
		Uploads: NewBackendWrapper(log, stripe.UploadsBackend, config.Retries),
	}

	return &stripeClient{
		charges:                     &charge.Client{B: backends.API, Key: key},
		creditNotes:                 &creditnote.Client{B: backends.API, Key: key},
		customerBalanceTransactions: &customerbalancetransaction.Client{B: backends.API, Key: key},
		customers:                   &customer.Client{B: backends.API, Key: key},
		invoiceItems:                &invoiceitem.Client{B: backends.API, Key: key},
		invoices:                    &invoice.Client{B: backends.API, Key: key},
		paymentMethods:              &paymentmethod.Client{B: backends.API, Key: key},
		paymentIntents:              &paymentintent.Client{B: backends.API, Key: key},
		setupIntents:                &setupintent.Client{B: backends.API, Key: key},
		promotionCodes:              &promotioncode.Client{B: backends.API, Key: key},
		taxIDs:                      &taxid.Client{B: backends.API, Key: key},
	}
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

	// log is passed to backend and used for the logging errors that are retried.
	log *zap.Logger
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
		log:      log,
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

	for retry := int64(0); ; retry++ {
		err := call()
		if err == nil {
			return nil
		}

		if !w.shouldRetry(retry, err) {
			return err
		}

		minBackoff := float64(w.retryCfg.InitialBackoff)
		maxBackoff := math.Min(
			float64(w.retryCfg.MaxBackoff),
			minBackoff*math.Pow(w.retryCfg.Multiplier, float64(retry)),
		)
		backoff := minBackoff + rand.Float64()*(maxBackoff-minBackoff)

		if !w.clock.Sleep(ctx, time.Duration(backoff)) {
			return ctx.Err()
		}

		w.log.Warn("retrying stripe request", zap.Error(err))
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
