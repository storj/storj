// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripepayments

import (
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
)

type Customer struct {
	ID      string
	Email   string
	Name    string
	Balance int64
}

type Customers struct {
}

func (customers *Customers) Create(description, name, email string) (string, error) {
	params := &stripe.CustomerParams{
		AccountBalance: stripe.Int64(0),
		Description:    stripe.String(description),
		Email:          stripe.String(email),
		Name:           stripe.String(name),
	}

	//params.SetSource("tok_1234")

	customer, err := customer.New(params)
	if err != nil {
		return "", err
	}

	return customer.ID, nil
}

func (customers *Customers) Get(id string) (Customer, error) {
	customer, err := customer.Get(id, nil)
	if err != nil {
		return Customer{}, err
	}

	return Customer{
		ID:      customer.ID,
		Email:   customer.Email,
		Name:    customer.Name,
		Balance: customer.Balance,
	}, nil
}

func (customers *Customers) List() (list []Customer) {
	params := &stripe.CustomerListParams{

	}

	iterator := customer.List(params)

	for iterator.Next() {
		currentCustomer := iterator.Customer()

		list = append(list, Customer{
			ID:      currentCustomer.ID,
			Balance: currentCustomer.Balance,
			Name:    currentCustomer.Name,
			Email:   currentCustomer.Email,
		})
	}

	return
}
