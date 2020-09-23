// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/customerbalancetransaction"
	"github.com/stripe/stripe-go/form"
	"github.com/stripe/stripe-go/invoice"
	"github.com/stripe/stripe-go/invoiceitem"
	"github.com/stripe/stripe-go/paymentmethod"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
)

// mocks synchronized map for caching mockStripeClient.
//
// The satellite has a Core part and API part which mostly duplicate each
// other. Each of them have a StripeClient instance. This is not a problem in
// production, because the stripeClient implementation is stateless and calls
// the Web API of the same Stripe backend. But it is a problem in test
// environments as the mockStripeClient is stateful - the data is stored in
// in-memory maps. Therefore, we need the Core and API parts share the same
// instance of mockStripeClient.
var mocks = struct {
	sync.Mutex
	m map[storj.NodeID]StripeClient
}{
	m: make(map[storj.NodeID]StripeClient),
}

// mockStripeClient Stripe client mock.
type mockStripeClient struct {
	customers                   *mockCustomers
	paymentMethods              *mockPaymentMethods
	invoices                    *mockInvoices
	invoiceItems                *mockInvoiceItems
	customerBalanceTransactions *mockCustomerBalanceTransactions
	charges                     *mockCharges
}

// NewStripeMock creates new Stripe client mock.
//
// A new mock is returned for each unique id. If this method is called multiple
// times with the same id, it will return the same mock instance for that id.
//
// If called by satellite component, the id param should be the peer.ID().
// If called by CLI tool, the id param should be a zero value, i.e. storj.NodeID{}.
// If called by satellitedb test case, the id param should be a random value,
// i.e. testrand.NodeID().
func NewStripeMock(id storj.NodeID) StripeClient {
	mocks.Lock()
	defer mocks.Unlock()

	mock, ok := mocks.m[id]
	if !ok {
		mock = &mockStripeClient{
			customers:                   newMockCustomers(),
			paymentMethods:              newMockPaymentMethods(),
			invoices:                    &mockInvoices{},
			invoiceItems:                &mockInvoiceItems{},
			customerBalanceTransactions: newMockCustomerBalanceTransactions(),
			charges:                     &mockCharges{},
		}
		mocks.m[id] = mock
	}

	return mock
}

func (m *mockStripeClient) Customers() StripeCustomers {
	return m.customers
}

func (m *mockStripeClient) PaymentMethods() StripePaymentMethods {
	return m.paymentMethods
}

func (m *mockStripeClient) Invoices() StripeInvoices {
	return m.invoices
}

func (m *mockStripeClient) InvoiceItems() StripeInvoiceItems {
	return m.invoiceItems
}

func (m *mockStripeClient) CustomerBalanceTransactions() StripeCustomerBalanceTransactions {
	return m.customerBalanceTransactions
}

func (m *mockStripeClient) Charges() StripeCharges {
	return m.charges
}

type mockCustomers struct {
	customers []*stripe.Customer
}

func newMockCustomers() *mockCustomers {
	return &mockCustomers{
		customers: make([]*stripe.Customer, 0, 5),
	}
}

func (m *mockCustomers) New(params *stripe.CustomerParams) (*stripe.Customer, error) {
	uuid, err := uuid.New()
	if err != nil {
		return nil, err
	}
	customer := &stripe.Customer{
		ID:    uuid.String(),
		Email: *params.Email,
		InvoiceSettings: &stripe.CustomerInvoiceSettings{
			DefaultPaymentMethod: &stripe.PaymentMethod{
				ID: "pm_card_mastercard",
			},
		},
	}

	mocks.Lock()
	defer mocks.Unlock()

	m.customers = append(m.customers, customer)
	return customer, nil
}

func (m *mockCustomers) Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	mocks.Lock()
	defer mocks.Unlock()

	for _, customer := range m.customers {
		if id == customer.ID {
			return customer, nil
		}
	}
	return nil, errors.New("customer not found")
}

func (m *mockCustomers) Update(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	customer, err := m.Get(id, nil)
	if err != nil {
		return nil, err
	}

	if params == nil {
		return customer, nil
	}

	mocks.Lock()
	defer mocks.Unlock()

	if params.Metadata != nil {
		customer.Metadata = params.Metadata
	}

	// TODO update customer with more params as necessary

	return customer, nil
}

type mockPaymentMethods struct {
	// attached contains a mapping of customerID to its paymentMethods
	attached map[string][]*stripe.PaymentMethod
	// unattached contains created but not attached paymentMethods
	unattached []*stripe.PaymentMethod
}

func newMockPaymentMethods() *mockPaymentMethods {
	return &mockPaymentMethods{
		attached: map[string][]*stripe.PaymentMethod{},
	}
}

func (m *mockPaymentMethods) List(listParams *stripe.PaymentMethodListParams) *paymentmethod.Iter {
	listMeta := stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(m.attached)),
	}
	return &paymentmethod.Iter{Iter: stripe.GetIter(nil, func(*stripe.Params, *form.Values) ([]interface{}, stripe.ListMeta, error) {
		mocks.Lock()
		defer mocks.Unlock()

		list, ok := m.attached[*listParams.Customer]
		if !ok {
			list = []*stripe.PaymentMethod{}
		}
		ret := make([]interface{}, len(list))

		for i, v := range list {
			ret[i] = v
		}

		return ret, listMeta, nil
	})}
}

func (m *mockPaymentMethods) New(params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error) {

	randID := testrand.BucketName()
	newMethod := &stripe.PaymentMethod{
		ID: fmt.Sprintf("pm_card_%s", randID),
		Card: &stripe.PaymentMethodCard{
			ExpMonth:    12,
			ExpYear:     2050,
			Brand:       "Mastercard",
			Last4:       "4444",
			Description: randID,
		},
		Type: stripe.PaymentMethodTypeCard,
	}

	mocks.Lock()
	defer mocks.Unlock()

	m.unattached = append(m.unattached, newMethod)

	return newMethod, nil
}

func (m *mockPaymentMethods) Attach(id string, params *stripe.PaymentMethodAttachParams) (*stripe.PaymentMethod, error) {
	var method *stripe.PaymentMethod

	mocks.Lock()
	defer mocks.Unlock()

	for _, candidate := range m.unattached {
		if candidate.ID == id {
			method = candidate
		}
	}
	attached, ok := m.attached[*params.Customer]
	if !ok {
		attached = []*stripe.PaymentMethod{}
	}
	m.attached[*params.Customer] = append(attached, method)
	return method, nil
}

func (m *mockPaymentMethods) Detach(id string, params *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
	var unattached *stripe.PaymentMethod

	mocks.Lock()
	defer mocks.Unlock()

	for user, userMethods := range m.attached {
		var remaining []*stripe.PaymentMethod
		for _, method := range userMethods {
			if id == method.ID {
				unattached = method
			} else {
				remaining = append(remaining, method)
			}
		}
		m.attached[user] = remaining
	}

	return unattached, nil
}

type mockInvoices struct {
}

func (m *mockInvoices) New(params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	return nil, nil
}

func (m *mockInvoices) List(listParams *stripe.InvoiceListParams) *invoice.Iter {
	return &invoice.Iter{Iter: &stripe.Iter{}}
}

func (m *mockInvoices) FinalizeInvoice(id string, params *stripe.InvoiceFinalizeParams) (*stripe.Invoice, error) {
	return nil, nil
}

type mockInvoiceItems struct {
}

func (m *mockInvoiceItems) New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error) {
	return nil, nil
}

func (m *mockInvoiceItems) List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter {
	return &invoiceitem.Iter{Iter: &stripe.Iter{}}
}

type mockCustomerBalanceTransactions struct {
	transactions map[string][]*stripe.CustomerBalanceTransaction
}

func newMockCustomerBalanceTransactions() *mockCustomerBalanceTransactions {
	return &mockCustomerBalanceTransactions{
		transactions: make(map[string][]*stripe.CustomerBalanceTransaction),
	}
}

func (m *mockCustomerBalanceTransactions) New(params *stripe.CustomerBalanceTransactionParams) (*stripe.CustomerBalanceTransaction, error) {
	tx := &stripe.CustomerBalanceTransaction{
		Type:        stripe.CustomerBalanceTransactionTypeAdjustment,
		Amount:      *params.Amount,
		Description: *params.Description,
		Metadata:    params.Metadata,
		Created:     time.Now().Unix(),
	}

	mocks.Lock()
	defer mocks.Unlock()

	m.transactions[*params.Customer] = append(m.transactions[*params.Customer], tx)

	return tx, nil
}

func (m *mockCustomerBalanceTransactions) List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter {
	mocks.Lock()
	defer mocks.Unlock()

	return &customerbalancetransaction.Iter{Iter: stripe.GetIter(listParams, func(p *stripe.Params, b *form.Values) ([]interface{}, stripe.ListMeta, error) {
		txs := m.transactions[*listParams.Customer]
		ret := make([]interface{}, len(txs))

		for i, v := range txs {
			ret[i] = v
		}

		listMeta := stripe.ListMeta{
			TotalCount: uint32(len(txs)),
		}

		return ret, listMeta, nil
	})}
}

type mockCharges struct {
}

func (m *mockCharges) List(listParams *stripe.ChargeListParams) *charge.Iter {
	return &charge.Iter{Iter: &stripe.Iter{}}
}
