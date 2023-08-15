// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"github.com/stripe/stripe-go/v72/customer"
	"github.com/stripe/stripe-go/v72/customerbalancetransaction"
	"github.com/stripe/stripe-go/v72/form"
	"github.com/stripe/stripe-go/v72/invoice"
	"github.com/stripe/stripe-go/v72/invoiceitem"
	"github.com/stripe/stripe-go/v72/paymentmethod"
	"github.com/stripe/stripe-go/v72/promotioncode"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

const (
	// MockCouponID1 is a coupon that stripe mock is aware of. Applying unknown coupons results in failure.
	MockCouponID1 = "c1"
	// MockCouponID2 is a coupon that stripe mock is aware of. Applying unknown coupons results in failure.
	MockCouponID2 = "c2"
	// MockCouponID3 is a coupon that stripe mock is aware of. Applying unknown coupons results in failure.
	MockCouponID3 = "c3"

	// MockInvoicesNewFailure can be passed to mockInvoices.New as `desc` argument to cause it to return
	// an error.
	MockInvoicesNewFailure = "mock_invoices_new_failure"

	// MockInvoicesPayFailure can be passed to mockInvoices.Pay as params.PaymentMethod to cause it to return
	// an error.
	MockInvoicesPayFailure = "mock_invoices_pay_failure"

	// MockInvoicesPaySuccess can be passed to mockInvoices.Pay as params.PaymentMethod to cause it to return
	// a paid invoice.
	MockInvoicesPaySuccess = "mock_invoices_pay_success"

	// TestPaymentMethodsNewFailure can be passed to creditCards.Add as the cardToken arg to cause
	// mockPaymentMethods.New to return an error.
	TestPaymentMethodsNewFailure = "test_payment_methods_new_failure"

	// TestPaymentMethodsAttachFailure can be passed to creditCards.Add as the cardToken arg to cause
	// mockPaymentMethods.Attach to return an error.
	TestPaymentMethodsAttachFailure = "test_payment_methods_attach_failure"

	// MockCBTXsNewFailure can be passed to mockCustomerBalanceTransactions.New as the `desc` argument to cause it
	// to return an error.
	MockCBTXsNewFailure = "mock_cbtxs_new_failure"
)

var (
	testPromoCodes = map[string]*stripe.PromotionCode{
		"promo1": {
			ID: "p1",
			Coupon: &stripe.Coupon{
				AmountOff: 500,
				Currency:  stripe.CurrencyUSD,
				Name:      "Test Promo Code 1",
				ID:        MockCouponID1,
			},
		},
		"promo2": {
			ID: "p2",
			Coupon: &stripe.Coupon{
				PercentOff: 50,
				Name:       "Test Promo Code 2",
				ID:         MockCouponID2,
			},
		},
		"promo3": {
			ID: "p3",
			Coupon: &stripe.Coupon{
				AmountOff: 100,
				Currency:  stripe.CurrencyUSD,
				Name:      "Test Promo Code 3",
				ID:        MockCouponID3,
			},
		},
	}
	promoIDs = map[string]*stripe.PromotionCode{
		"p1": testPromoCodes["promo1"],
		"p2": testPromoCodes["promo2"],
	}
	mockCoupons = map[string]*stripe.Coupon{
		MockCouponID1: testPromoCodes["promo1"].Coupon,
		MockCouponID2: testPromoCodes["promo2"].Coupon,
		MockCouponID3: testPromoCodes["promo3"].Coupon,
	}
)

// mockStripeState Stripe client mock.
type mockStripeState struct {
	mu sync.Mutex

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
func NewStripeMock(customersDB CustomersDB, usersDB console.Users) Client {
	state := &mockStripeState{}
	state.customers = &mockCustomersState{}
	state.paymentMethods = newMockPaymentMethods(state)
	state.invoiceItems = newMockInvoiceItems(state)
	state.invoices = newMockInvoices(state, state.invoiceItems)
	state.customerBalanceTransactions = newMockCustomerBalanceTransactions(state)
	state.charges = &mockCharges{}
	state.promoCodes = newMockPromoCodes(state)
	state.creditNotes = newMockCreditNotes(state)

	return &mockStripeClient{
		customersDB:     customersDB,
		usersDB:         usersDB,
		mockStripeState: state,
	}
}

func (m *mockStripeClient) Customers() Customers {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &mockCustomers{
		root: m.mockStripeState,

		customersDB: m.customersDB,
		usersDB:     m.usersDB,
		state:       m.customers,
		coupons:     mockCoupons,
	}
}

func (m *mockStripeClient) PaymentMethods() PaymentMethods {
	return m.paymentMethods
}

func (m *mockStripeClient) Invoices() Invoices {
	return m.invoices
}

func (m *mockStripeClient) InvoiceItems() InvoiceItems {
	return m.invoiceItems
}

func (m *mockStripeClient) CustomerBalanceTransactions() CustomerBalanceTransactions {
	return m.customerBalanceTransactions
}

func (m *mockStripeClient) Charges() Charges {
	return m.charges
}

func (m *mockStripeClient) PromoCodes() PromoCodes {
	return m.promoCodes
}

func (m *mockStripeClient) CreditNotes() CreditNotes {
	return m.creditNotes
}

type mockCustomers struct {
	root *mockStripeState

	customersDB CustomersDB
	usersDB     console.Users
	state       *mockCustomersState
	coupons     map[string]*stripe.Coupon
}

func (m *mockCustomers) List(listParams *stripe.CustomerListParams) *customer.Iter {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	return &customer.Iter{
		Iter: stripe.GetIter(listParams, func(p *stripe.Params, vals *form.Values) ([]interface{}, stripe.ListContainer, error) {
			var customers []interface{}
			for _, cus := range m.state.customers {
				customers = append(customers, cus)
			}
			return customers, newListContainer(&stripe.ListMeta{}), nil
		}),
	}
}

type mockCustomersState struct {
	customers   []*stripe.Customer
	repopulated bool
}

// The Stripe Client Mock is in-memory so all data is lost when the satellite is stopped.
// We need to repopulate the mock on every restart to ensure that requests to the mock
// for existing users won't fail with errors like "customer not found".
func (m *mockCustomers) repopulate() error {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	if !m.state.repopulated {
		const limit = 25
		ctx := context.TODO()

		cusPage, err := m.customersDB.List(ctx, uuid.UUID{}, limit, time.Now())
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
			cusPage, err = m.customersDB.List(ctx, cusPage.Cursor, limit, time.Now())
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

	if params.Coupon != nil && *params.Coupon != "" {
		c, ok := m.coupons[*params.Coupon]
		if !ok {
			return nil, &stripe.Error{}
		}
		customer.Discount = &stripe.Discount{Coupon: mockCoupons[c.ID]}
	}

	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	m.state.customers = append(m.state.customers, customer)
	return customer, nil
}

func (m *mockCustomers) Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
	if err := m.repopulate(); err != nil {
		return nil, err
	}

	m.root.mu.Lock()
	defer m.root.mu.Unlock()

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

	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	if params.Metadata != nil {
		customer.Metadata = params.Metadata
	}
	if params.PromotionCode != nil && promoIDs[*params.PromotionCode] != nil {
		customer.Discount = &stripe.Discount{Coupon: promoIDs[*params.PromotionCode].Coupon}
	}
	if params.Coupon != nil {
		c, ok := m.coupons[*params.Coupon]
		if !ok {
			return nil, &stripe.Error{}
		}
		customer.Discount = &stripe.Discount{Coupon: &stripe.Coupon{ID: c.ID}}
	}
	if params.Balance != nil {
		customer.Balance = *params.Balance
	}
	if params.InvoiceSettings != nil {
		if params.InvoiceSettings.DefaultPaymentMethod != nil {
			customer.InvoiceSettings = &stripe.CustomerInvoiceSettings{
				DefaultPaymentMethod: &stripe.PaymentMethod{
					ID: *params.InvoiceSettings.DefaultPaymentMethod,
				},
			}
		}
	}
	// TODO update customer with more params as necessary

	return customer, nil
}

type mockPaymentMethods struct {
	root *mockStripeState
	// attached contains a mapping of customerID to its paymentMethods
	attached map[string][]*stripe.PaymentMethod
	// unattached contains created but not attached paymentMethods
	unattached []*stripe.PaymentMethod
}

func newMockPaymentMethods(root *mockStripeState) *mockPaymentMethods {
	return &mockPaymentMethods{
		root:     root,
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
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

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
	id := fmt.Sprintf("pm_card_%s", randID)
	if params.Card.Token != nil {
		switch *params.Card.Token {
		case TestPaymentMethodsNewFailure:
			return nil, &stripe.Error{}
		case TestPaymentMethodsAttachFailure:
			id = TestPaymentMethodsAttachFailure
		case MockInvoicesPayFailure:
			id = MockInvoicesPayFailure
		}
	}

	newMethod := &stripe.PaymentMethod{
		ID: id,
		Card: &stripe.PaymentMethodCard{
			ExpMonth:    12,
			ExpYear:     2050,
			Brand:       "Mastercard",
			Last4:       "4444",
			Description: randID,
		},
		Type: stripe.PaymentMethodTypeCard,
	}

	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	m.unattached = append(m.unattached, newMethod)

	return newMethod, nil
}

func (m *mockPaymentMethods) Attach(id string, params *stripe.PaymentMethodAttachParams) (*stripe.PaymentMethod, error) {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	var method *stripe.PaymentMethod
	for _, candidate := range m.unattached {
		if candidate.ID == id {
			if id == TestPaymentMethodsAttachFailure {
				return nil, &stripe.Error{}
			}
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
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

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
	root *mockStripeState

	invoices     map[string][]*stripe.Invoice
	invoiceItems *mockInvoiceItems
}

func (m *mockInvoices) MarkUncollectible(id string, params *stripe.InvoiceMarkUncollectibleParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for _, invoice := range invoices {
			if invoice.ID == id {
				invoice.Status = stripe.InvoiceStatusUncollectible
				return invoice, nil
			}
		}
	}

	return nil, errors.New("invoice not found")
}

func (m *mockInvoices) VoidInvoice(id string, params *stripe.InvoiceVoidParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for _, invoice := range invoices {
			if invoice.ID == id {
				invoice.Status = stripe.InvoiceStatusVoid
				return invoice, nil
			}
		}
	}

	return nil, errors.New("invoice not found")
}

func newMockInvoices(root *mockStripeState, invoiceItems *mockInvoiceItems) *mockInvoices {
	return &mockInvoices{
		root:         root,
		invoices:     make(map[string][]*stripe.Invoice),
		invoiceItems: invoiceItems,
	}
}

func (m *mockInvoices) New(params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	items, ok := m.invoiceItems.items[*params.Customer]
	if !ok || len(items) == 0 {
		if params.PendingInvoiceItemsBehavior == nil || *params.PendingInvoiceItemsBehavior != "exclude" {
			return nil, &stripe.Error{Code: stripe.ErrorCodeInvoiceNoCustomerLineItems}
		}
	}

	due := int64(0)
	if params.DueDate != nil {
		due = *params.DueDate
	}

	amountDue := int64(0)
	lineData := make([]*stripe.InvoiceLine, 0, len(params.InvoiceItems))
	for _, item := range params.InvoiceItems {
		lineData = append(lineData, &stripe.InvoiceLine{
			InvoiceItem: *item.InvoiceItem,
			Amount:      *item.Amount,
		})
		amountDue += *item.Amount
	}

	var desc string
	if params.Description != nil {
		if *params.Description == MockInvoicesNewFailure {
			return nil, &stripe.Error{}
		}
		desc = *params.Description
	}

	invoice := &stripe.Invoice{
		ID:          "in_" + string(testrand.RandAlphaNumeric(25)),
		Customer:    &stripe.Customer{ID: *params.Customer},
		DueDate:     due,
		Status:      stripe.InvoiceStatusDraft,
		Description: desc,
		Lines: &stripe.InvoiceLineList{
			Data: lineData,
		},
		AmountDue:       amountDue,
		AmountRemaining: amountDue,
		Total:           amountDue,
	}
	if params.DefaultPaymentMethod != nil {
		invoice.DefaultPaymentMethod = &stripe.PaymentMethod{ID: *params.DefaultPaymentMethod}
	}

	m.invoices[*params.Customer] = append(m.invoices[*params.Customer], invoice)
	for _, item := range items {
		if item.Invoice == nil {
			item.Invoice = invoice
		}
	}

	return invoice, nil
}

func (m *mockInvoices) List(listParams *stripe.InvoiceListParams) *invoice.Iter {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	listMeta := &stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(m.invoices)),
	}
	lc := newListContainer(listMeta)

	query := stripe.Query(func(*stripe.Params, *form.Values) (ret []interface{}, _ stripe.ListContainer, _ error) {
		if listParams.Customer == nil && listParams.Status != nil {
			// filter by status
			for _, invoices := range m.invoices {
				for _, inv := range invoices {
					if inv.Status == stripe.InvoiceStatus(*listParams.Status) {
						ret = append(ret, inv)
					}
				}
			}
		} else if listParams.Customer != nil && listParams.Status != nil {
			// filter by status and customer
			for _, invoices := range m.invoices {
				for _, inv := range invoices {
					if inv.Status == stripe.InvoiceStatus(*listParams.Status) && inv.Customer.ID == *listParams.Customer {
						ret = append(ret, inv)
					}
				}
			}
		} else if listParams.Customer == nil {
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

// FinalizeInvoice forwards the invoice's status from draft to open.
func (m *mockInvoices) FinalizeInvoice(id string, params *stripe.InvoiceFinalizeParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for _, invoice := range invoices {
			if invoice.ID == id && invoice.Status == stripe.InvoiceStatusDraft {
				invoice.Status = stripe.InvoiceStatusOpen
				return invoice, nil
			}
		}
	}
	return nil, &stripe.Error{}
}

func (m *mockInvoices) Pay(id string, params *stripe.InvoicePayParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for _, invoice := range invoices {
			if invoice.ID == id {
				if params.PaymentMethod != nil {
					if *params.PaymentMethod == MockInvoicesPayFailure {
						invoice.Status = stripe.InvoiceStatusOpen
						return invoice, &stripe.Error{}
					}
					if *params.PaymentMethod == MockInvoicesPaySuccess {
						invoice.Status = stripe.InvoiceStatusPaid
						invoice.AmountRemaining = 0
						return invoice, nil
					}
				} else if invoice.DefaultPaymentMethod != nil {
					if invoice.DefaultPaymentMethod.ID == MockInvoicesPaySuccess {
						invoice.Status = stripe.InvoiceStatusPaid
						invoice.AmountRemaining = 0
						return invoice, nil
					}
					if invoice.DefaultPaymentMethod.ID == MockInvoicesNewFailure {
						invoice.Status = stripe.InvoiceStatusOpen
						return invoice, &stripe.Error{}
					}
				} else if invoice.AmountRemaining == 0 || (params.PaidOutOfBand != nil && *params.PaidOutOfBand) {
					invoice.Status = stripe.InvoiceStatusPaid
					invoice.AmountRemaining = 0
				}
				return invoice, nil
			}
		}
	}
	return nil, &stripe.Error{}
}

func (m *mockInvoices) Del(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for i, invoice := range invoices {
			if invoice.ID == id {
				m.invoices[invoice.Customer.ID] = append(m.invoices[invoice.Customer.ID][:i], m.invoices[invoice.Customer.ID][i+1:]...)
				return invoice, nil
			}
		}
	}
	return nil, nil
}

func (m *mockInvoices) Get(id string, params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	for _, invoices := range m.invoices {
		for _, inv := range invoices {
			if inv.ID == id {
				items, ok := m.invoiceItems.items[inv.Customer.ID]
				if ok {
					amountDue := int64(0)
					lineData := make([]*stripe.InvoiceLine, 0, len(params.InvoiceItems))
					for _, item := range items {
						if item.Invoice != inv {
							continue
						}
						lineData = append(lineData, &stripe.InvoiceLine{
							InvoiceItem: item.ID,
							Amount:      item.Amount,
						})
						amountDue += item.Amount
					}
					inv.Lines.Data = lineData
					inv.Total = amountDue
				}
				return inv, nil
			}
		}
	}
	return nil, nil
}

type mockInvoiceItems struct {
	root  *mockStripeState
	items map[string][]*stripe.InvoiceItem
}

func newMockInvoiceItems(root *mockStripeState) *mockInvoiceItems {
	return &mockInvoiceItems{
		root:  root,
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
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	item := &stripe.InvoiceItem{
		Metadata: params.Metadata,
	}
	if params.Description != nil {
		item.Description = *params.Description
	}
	if params.UnitAmountDecimal != nil {
		item.UnitAmountDecimal = *params.UnitAmountDecimal
	}
	if params.UnitAmount != nil {
		item.UnitAmount = *params.UnitAmount
	}
	if params.Amount != nil {
		item.Amount = *params.Amount
	}
	m.items[*params.Customer] = append(m.items[*params.Customer], item)

	return item, nil
}

func (m *mockInvoiceItems) List(listParams *stripe.InvoiceItemListParams) *invoiceitem.Iter {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

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
	root         *mockStripeState
	transactions map[string][]*stripe.CustomerBalanceTransaction
}

func newMockCustomerBalanceTransactions(root *mockStripeState) *mockCustomerBalanceTransactions {
	return &mockCustomerBalanceTransactions{
		root:         root,
		transactions: make(map[string][]*stripe.CustomerBalanceTransaction),
	}
}

func (m *mockCustomerBalanceTransactions) New(params *stripe.CustomerBalanceTransactionParams) (*stripe.CustomerBalanceTransaction, error) {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	if params.Description != nil {
		if *params.Description == MockCBTXsNewFailure {
			return nil, &stripe.Error{}
		}
	}
	tx := &stripe.CustomerBalanceTransaction{
		Type:        stripe.CustomerBalanceTransactionTypeAdjustment,
		Amount:      *params.Amount,
		Description: *params.Description,
		Metadata:    params.Metadata,
		Created:     time.Now().Unix(),
	}

	m.transactions[*params.Customer] = append(m.transactions[*params.Customer], tx)

	for _, v := range m.root.customers.customers {
		if v.ID == *params.Customer {
			v.Balance += *params.Amount
			tx.EndingBalance = v.Balance
		}
	}
	return tx, nil
}

func (m *mockCustomerBalanceTransactions) List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	query := stripe.Query(func(p *stripe.Params, b *form.Values) ([]interface{}, stripe.ListContainer, error) {
		txs := m.transactions[*listParams.Customer]
		ret := make([]interface{}, len(txs))

		for i, v := range txs {
			// stripe returns list of transactions ordered by most recent, so reverse the array.
			ret[len(txs)-1-i] = v
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
	root *mockStripeState

	promoCodes map[string]*stripe.PromotionCode
}

func newMockPromoCodes(root *mockStripeState) *mockPromoCodes {
	return &mockPromoCodes{
		root:       root,
		promoCodes: testPromoCodes,
	}
}

func (m *mockPromoCodes) List(params *stripe.PromotionCodeListParams) *promotioncode.Iter {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

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
	root *mockStripeState

	CreditNotes map[string]*stripe.CreditNote
}

func newMockCreditNotes(root *mockStripeState) *mockCreditNotes {
	return &mockCreditNotes{
		root: root,
	}
}

func (m mockCreditNotes) New(params *stripe.CreditNoteParams) (*stripe.CreditNote, error) {
	m.root.mu.Lock()
	defer m.root.mu.Unlock()

	item := &stripe.CreditNote{}

	if params.Invoice != nil {
		item.ID = *params.Invoice
	}
	if params.Memo != nil {
		item.Memo = *params.Memo
	}
	for _, invoices := range m.root.invoices.invoices {
		for _, invoice := range invoices {
			if invoice.ID == *params.Invoice {
				invoice.AmountRemaining -= *params.Lines[0].UnitAmount
				invoice.AmountDue -= *params.Lines[0].UnitAmount
				invoice.Lines.Data[0].Amount -= *params.Lines[0].UnitAmount
				if invoice.AmountRemaining <= 0 {
					invoice.Status = stripe.InvoiceStatusPaid
				}
			}
		}
	}
	return item, nil
}
