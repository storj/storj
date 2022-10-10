// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"github.com/stripe/stripe-go/v72/customerbalancetransaction"
	"github.com/stripe/stripe-go/v72/form"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/invoiceitem"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/promotioncode"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
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
	m map[storj.NodeID]*mockStripeState
}{
	m: make(map[storj.NodeID]*mockStripeState),
}

var (
	testPromoCodes = map[string]*stripe.PromotionCode{
		"promo1": {
			ID: "p1",
			Coupon: &stripe.Coupon{
				AmountOff: 500,
				Currency:  stripe.CurrencyUSD,
				Name:      "Test Promo Code 1",
				ID:        "c1",
			},
		},
		"promo2": {
			ID: "p2",
			Coupon: &stripe.Coupon{
				PercentOff: 50,
				Name:       "Test Promo Code 2",
				ID:         "c2",
			},
		},
	}
	promoIDs = map[string]*stripe.PromotionCode{
		"p1": testPromoCodes["promo1"],
		"p2": testPromoCodes["promo2"],
	}
	couponIDs = map[string]*stripe.Coupon{
		"c1": testPromoCodes["promo1"].Coupon,
		"c2": testPromoCodes["promo2"].Coupon,
	}
)

// mockStripeState Stripe client mock.
type mockStripeState struct {
	customers                   *mockCustomersState
	paymentMethods              *mockPaymentMethods
	invoices                    *mockInvoices
	invoiceItems                *mockInvoiceItems
	customerBalanceTransactions *mockCustomerBalanceTransactions
	charges                     *mockCharges
	promoCodes                  *mockPromoCodes
	creditNotes                 *mockCreditNotes
}

type mockStripeClient struct {
	customersDB CustomersDB
	usersDB     console.Users
	*mockStripeState
}

// mockEmptyQuery is a query with no results.
var mockEmptyQuery = stripe.Query(func(*stripe.Params, *form.Values) ([]interface{}, stripe.ListContainer, error) {
	return nil, newListContainer(&stripe.ListMeta{}), nil
})

// NewStripeMock creates new Stripe client mock.
//
// A new mock is returned for each unique id. If this method is called multiple
// times with the same id, it will return the same mock instance for that id.
//
// If called by satellite component, the id param should be the peer.ID().
// If called by CLI tool, the id param should be a zero value, i.e. storj.NodeID{}.
// If called by satellitedb test case, the id param should be a random value,
// i.e. testrand.NodeID().
func NewStripeMock(id storj.NodeID, customersDB CustomersDB, usersDB console.Users) StripeClient {
	mocks.Lock()
	defer mocks.Unlock()

	state, ok := mocks.m[id]
	if !ok {
		state = &mockStripeState{
			customers:                   &mockCustomersState{},
			paymentMethods:              newMockPaymentMethods(),
			invoices:                    newMockInvoices(),
			invoiceItems:                newMockInvoiceItems(),
			customerBalanceTransactions: newMockCustomerBalanceTransactions(),
			charges:                     &mockCharges{},
			promoCodes: &mockPromoCodes{
				promoCodes: testPromoCodes,
			},
		}
		state.invoices.invoiceItems = state.invoiceItems
		mocks.m[id] = state
	}

	return &mockStripeClient{
		customersDB:     customersDB,
		usersDB:         usersDB,
		mockStripeState: state,
	}
}

func (m *mockStripeClient) Customers() StripeCustomers {
	mocks.Lock()
	defer mocks.Unlock()

	return &mockCustomers{
		customersDB: m.customersDB,
		usersDB:     m.usersDB,
		state:       m.customers,
	}
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

func (m *mockStripeClient) PromoCodes() StripePromoCodes {
	return m.promoCodes
}

func (m *mockStripeClient) CreditNotes() StripeCreditNotes {
	return m.creditNotes
}

type mockCustomers struct {
	customersDB CustomersDB
	usersDB     console.Users
	state       *mockCustomersState
}

type mockCustomersState struct {
	customers   []*stripe.Customer
	repopulated bool
}

// The Stripe Client Mock is in-memory so all data is lost when the satellite is stopped.
// We need to repopulate the mock on every restart to ensure that requests to the mock
// for existing users won't fail with errors like "customer not found".
func (m *mockCustomers) repopulate() error {
	mocks.Lock()
	defer mocks.Unlock()

	if !m.state.repopulated {
		const limit = 25
		ctx := context.TODO()

		cusPage, err := m.customersDB.List(ctx, 0, limit, time.Now())
		if err != nil {
			return err
		}
		for _, cus := range cusPage.Customers {
			user, err := m.usersDB.Get(ctx, cus.UserID)
			if err != nil {
				return err
			}
			m.state.customers = append(m.state.customers, newMockCustomer(cus.ID, user.Email))
		}

		for cusPage.Next {
			cusPage, err = m.customersDB.List(ctx, cusPage.NextOffset, limit, time.Now())
			if err != nil {
				return err
			}
			for _, cus := range cusPage.Customers {
				user, err := m.usersDB.Get(ctx, cus.UserID)
				if err != nil {
					return err
				}
				m.state.customers = append(m.state.customers, newMockCustomer(cus.ID, user.Email))
			}
		}

		m.state.repopulated = true
	}

	return nil
}

func newMockCustomer(id, email string) *stripe.Customer {
	return &stripe.Customer{
		ID:    id,
		Email: email,
		InvoiceSettings: &stripe.CustomerInvoiceSettings{
			DefaultPaymentMethod: &stripe.PaymentMethod{
				ID: "pm_card_mastercard",
			},
		},
	}
}

func (m *mockCustomers) New(params *stripe.CustomerParams) (*stripe.Customer, error) {
	if err := m.repopulate(); err != nil {
		return nil, err
	}

	uuid, err := uuid.New()
	if err != nil {
		return nil, err
	}

	customer := newMockCustomer(uuid.String(), *params.Email)

	if params.PromotionCode != nil && promoIDs[*params.PromotionCode] != nil {
		customer.Discount = &stripe.Discount{Coupon: promoIDs[*params.PromotionCode].Coupon}
	}

	if params.Coupon != nil {
		customer.Discount = &stripe.Discount{Coupon: couponIDs[*params.Coupon]}
	}

	mocks.Lock()
	defer mocks.Unlock()

	m.state.customers = append(m.state.customers, customer)
	return customer, nil
}

func (m *mockCustomers) Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	if err := m.repopulate(); err != nil {
		return nil, err
	}

	mocks.Lock()
	defer mocks.Unlock()

	for _, customer := range m.state.customers {
		if id == customer.ID {
			return customer, nil
		}
	}

	return nil, errors.New("customer not found")
}

func (m *mockCustomers) Update(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	if err := m.repopulate(); err != nil {
		return nil, err
	}

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
	if params.PromotionCode != nil && promoIDs[*params.PromotionCode] != nil {
		customer.Discount = &stripe.Discount{Coupon: promoIDs[*params.PromotionCode].Coupon}
	}
	if params.Coupon != nil {
		customer.Discount = &stripe.Discount{Coupon: &stripe.Coupon{ID: *params.Coupon}}
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

// listContainer implements Stripe's ListContainer interface.
type listContainer struct {
	listMeta *stripe.ListMeta
}

func newListContainer(meta *stripe.ListMeta) *listContainer {
	return &listContainer{listMeta: meta}
}

func (c *listContainer) GetListMeta() *stripe.ListMeta {
	return c.listMeta
}

func (m *mockPaymentMethods) List(listParams *stripe.PaymentMethodListParams) *paymentmethod.Iter {
	mocks.Lock()
	defer mocks.Unlock()

	listMeta := &stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(m.attached)),
	}
	lc := newListContainer(listMeta)

	query := stripe.Query(func(*stripe.Params, *form.Values) ([]interface{}, stripe.ListContainer, error) {
		list, ok := m.attached[*listParams.Customer]
		if !ok {
			list = []*stripe.PaymentMethod{}
		}
		ret := make([]interface{}, len(list))

		for i, v := range list {
			ret[i] = v
		}

		return ret, lc, nil
	})
	return &paymentmethod.Iter{Iter: stripe.GetIter(nil, query)}
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
	mocks.Lock()
	defer mocks.Unlock()

	var method *stripe.PaymentMethod
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
	mocks.Lock()
	defer mocks.Unlock()

	var unattached *stripe.PaymentMethod
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
	invoices     map[string][]*stripe.Invoice
	invoiceItems *mockInvoiceItems
}

func newMockInvoices() *mockInvoices {
	return &mockInvoices{
		invoices: make(map[string][]*stripe.Invoice),
	}
}

func (m *mockInvoices) New(params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	mocks.Lock()
	defer mocks.Unlock()

	invoice := &stripe.Invoice{ID: "in_" + string(testrand.RandAlphaNumeric(25))}
	m.invoices[*params.Customer] = append(m.invoices[*params.Customer], invoice)

	if items, ok := m.invoiceItems.items[*params.Customer]; ok {
		for _, item := range items {
			if item.Invoice == nil {
				item.Invoice = invoice
			}
		}
	}

	return invoice, nil
}

func (m *mockInvoices) List(listParams *stripe.InvoiceListParams) *invoice.Iter {
	mocks.Lock()
	defer mocks.Unlock()

	listMeta := &stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(m.invoices)),
	}
	lc := newListContainer(listMeta)

	query := stripe.Query(func(*stripe.Params, *form.Values) (ret []interface{}, _ stripe.ListContainer, _ error) {
		if listParams.Customer == nil {
			for _, invoices := range m.invoices {
				for _, invoice := range invoices {
					ret = append(ret, invoice)
				}
			}
		} else if list, ok := m.invoices[*listParams.Customer]; ok {
			for _, invoice := range list {
				ret = append(ret, invoice)
			}
		}

		return ret, lc, nil
	})
	return &invoice.Iter{Iter: stripe.GetIter(nil, query)}
}

func (m *mockInvoices) Update(id string, params *stripe.InvoiceParams) (invoice *stripe.Invoice, err error) {
	for _, invoices := range m.invoices {
		for _, invoice := range invoices {
			if invoice.ID == id {
				return invoice, nil
			}
		}
	}

	return nil, errors.New("invoice not found")
}

func (m *mockInvoices) FinalizeInvoice(id string, params *stripe.InvoiceFinalizeParams) (*stripe.Invoice, error) {
	return nil, nil
}

func (m *mockInvoices) Pay(id string, params *stripe.InvoicePayParams) (*stripe.Invoice, error) {
	return nil, nil
}

type mockInvoiceItems struct {
	items map[string][]*stripe.InvoiceItem
}

func newMockInvoiceItems() *mockInvoiceItems {
	return &mockInvoiceItems{
		items: make(map[string][]*stripe.InvoiceItem),
	}
}

func (m *mockInvoiceItems) Update(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error) {
	return nil, nil
}

func (m *mockInvoiceItems) Del(id string, params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error) {
	return nil, nil
}

func (m *mockInvoiceItems) New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error) {
	mocks.Lock()
	defer mocks.Unlock()

	item := &stripe.InvoiceItem{
		Metadata: params.Metadata,
	}
	m.items[*params.Customer] = append(m.items[*params.Customer], item)

	return item, nil
}

func (m *mockInvoiceItems) List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter {
	mocks.Lock()
	defer mocks.Unlock()

	listMeta := &stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(m.items)),
	}
	lc := newListContainer(listMeta)

	query := stripe.Query(func(*stripe.Params, *form.Values) ([]interface{}, stripe.ListContainer, error) {
		list, ok := m.items[*listParams.Customer]
		if !ok {
			list = []*stripe.InvoiceItem{}
		}
		ret := make([]interface{}, len(list))

		for i, v := range list {
			ret[i] = v
		}

		return ret, lc, nil
	})
	return &invoiceitem.Iter{Iter: stripe.GetIter(nil, query)}
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

	query := stripe.Query(func(p *stripe.Params, b *form.Values) ([]interface{}, stripe.ListContainer, error) {
		txs := m.transactions[*listParams.Customer]
		ret := make([]interface{}, len(txs))

		for i, v := range txs {
			ret[i] = v
		}

		listMeta := &stripe.ListMeta{
			TotalCount: uint32(len(txs)),
		}

		lc := newListContainer(listMeta)

		return ret, lc, nil
	})

	return &customerbalancetransaction.Iter{Iter: stripe.GetIter(listParams, query)}
}

type mockCharges struct {
}

func (m *mockCharges) List(listParams *stripe.ChargeListParams) *charge.Iter {
	return &charge.Iter{Iter: stripe.GetIter(listParams, mockEmptyQuery)}
}

type mockPromoCodes struct {
	promoCodes map[string]*stripe.PromotionCode
}

func (m *mockPromoCodes) List(params *stripe.PromotionCodeListParams) *promotioncode.Iter {
	mocks.Lock()
	defer mocks.Unlock()

	query := stripe.Query(func(p *stripe.Params, b *form.Values) ([]interface{}, stripe.ListContainer, error) {
		promoCode := m.promoCodes[*params.Code]
		if promoCode == nil {
			return make([]interface{}, 0), &stripe.ListMeta{TotalCount: 0}, nil
		}
		ret := make([]interface{}, 1)
		ret[0] = promoCode

		listMeta := &stripe.ListMeta{
			TotalCount: 1,
		}

		lc := newListContainer(listMeta)

		return ret, lc, nil
	})

	return &promotioncode.Iter{Iter: stripe.GetIter(params, query)}
}

type mockCreditNotes struct {
}

func (m mockCreditNotes) New(params *stripe.CreditNoteParams) (*stripe.CreditNote, error) {
	return nil, nil
}
