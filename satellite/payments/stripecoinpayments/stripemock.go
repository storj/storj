// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"errors"
	"time"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/customerbalancetransaction"
	"github.com/stripe/stripe-go/form"
	"github.com/stripe/stripe-go/invoice"
	"github.com/stripe/stripe-go/paymentmethod"

	"storj.io/common/uuid"
)

// MockStripeClient Stripe client mock.
type mockStripeClient struct {
	customers                   *mockCustomers
	paymentMethods              *mockPaymentMethods
	invoices                    *mockInvoices
	invoiceItems                *mockInvoiceItems
	customerBalanceTransactions *mockCustomerBalanceTransactions
	charges                     *mockCharges
}

// NewStripeMock creates new Stripe client mock.
func NewStripeMock() StripeClient {
	return &mockStripeClient{
		customers:                   newMockCustomers(),
		paymentMethods:              &mockPaymentMethods{},
		invoices:                    &mockInvoices{},
		invoiceItems:                &mockInvoiceItems{},
		customerBalanceTransactions: newMockCustomerBalanceTransactions(),
		charges:                     &mockCharges{},
	}
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
	m.customers = append(m.customers, customer)
	return customer, nil
}

func (m *mockCustomers) Get(id string, params *stripe.CustomerParams) (*stripe.Customer, error) {
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

	// TODO add customer updating according to params
	return customer, nil
}

type mockPaymentMethods struct {
}

func (m *mockPaymentMethods) List(listParams *stripe.PaymentMethodListParams) *paymentmethod.Iter {
	values := []interface{}{
		&stripe.PaymentMethod{
			ID: "pm_card_mastercard",
			Card: &stripe.PaymentMethodCard{
				ExpMonth: 12,
				ExpYear:  2050,
				Brand:    "Mastercard",
				Last4:    "4444",
			},
		},
	}
	listMeta := stripe.ListMeta{
		HasMore:    false,
		TotalCount: uint32(len(values)),
	}
	return &paymentmethod.Iter{Iter: stripe.GetIter(nil, func(*stripe.Params, *form.Values) ([]interface{}, stripe.ListMeta, error) {
		return values, listMeta, nil
	})}
}

func (m *mockPaymentMethods) New(params *stripe.PaymentMethodParams) (*stripe.PaymentMethod, error) {
	return nil, nil
}

func (m *mockPaymentMethods) Attach(id string, params *stripe.PaymentMethodAttachParams) (*stripe.PaymentMethod, error) {
	return nil, nil
}

func (m *mockPaymentMethods) Detach(id string, params *stripe.PaymentMethodDetachParams) (*stripe.PaymentMethod, error) {
	return nil, nil
}

type mockInvoices struct {
}

func (m *mockInvoices) New(params *stripe.InvoiceParams) (*stripe.Invoice, error) {
	return nil, nil
}

func (m *mockInvoices) List(listParams *stripe.InvoiceListParams) *invoice.Iter {
	return &invoice.Iter{Iter: &stripe.Iter{}}
}

type mockInvoiceItems struct {
}

func (m *mockInvoiceItems) New(params *stripe.InvoiceItemParams) (*stripe.InvoiceItem, error) {
	return nil, nil
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

	m.transactions[*params.Customer] = append(m.transactions[*params.Customer], tx)

	return tx, nil
}

func (m *mockCustomerBalanceTransactions) List(listParams *stripe.CustomerBalanceTransactionListParams) *customerbalancetransaction.Iter {
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
