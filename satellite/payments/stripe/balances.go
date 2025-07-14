// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

type balances struct {
	service *Service
}

// ApplyCredit applies a credit of `amount` to the user's stripe balance with a description of `desc`.
func (balances *balances) ApplyCredit(ctx context.Context, userID uuid.UUID, amount int64, desc, idempotencyKey string) (b *payments.Balance, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := balances.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.CustomerBalanceTransactionParams{
		Customer:    stripe.String(customerID),
		Description: stripe.String(desc),
		Amount:      stripe.Int64(-amount),
		Currency:    stripe.String(string(stripe.CurrencyUSD)),
	}

	if balances.service.stripeConfig.UseIdempotency && idempotencyKey != "" {
		params.SetIdempotencyKey(idempotencyKey)
	}

	// NB: In stripe a negative amount means the customer is owed money.
	cbtx, err := balances.service.stripeClient.CustomerBalanceTransactions().New(params)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &payments.Balance{
		Credits: decimal.NewFromInt(-cbtx.EndingBalance),
	}, nil
}

func (balances *balances) ListTransactions(ctx context.Context, userID uuid.UUID) (_ []payments.BalanceTransaction, err error) {
	defer mon.Task()(&ctx)(&err)

	customerID, err := balances.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var list []payments.BalanceTransaction
	iter := balances.service.stripeClient.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{
		Customer: stripe.String(customerID),
	})
	for iter.Next() {
		stripeCBTX := iter.CustomerBalanceTransaction()
		if stripeCBTX != nil {
			list = append(list, payments.BalanceTransaction{
				ID:          stripeCBTX.ID,
				Amount:      -stripeCBTX.Amount,
				Description: stripeCBTX.Description,
			})
		}
	}
	if err = iter.Err(); err != nil {
		return nil, Error.Wrap(err)
	}
	return list, nil
}

// Get returns an integer amount in cents that represents the current balance of payment account.
func (balances *balances) Get(ctx context.Context, userID uuid.UUID) (_ payments.Balance, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	b, err := balances.service.billingDB.GetBalance(ctx, userID)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	customerID, err := balances.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}}
	customer, err := balances.service.stripeClient.Customers().Get(customerID, params)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	// customer.Balance is negative if the user has a balance with us.
	// https://stripe.com/docs/api/customers/object#customer_object-balance
	return payments.Balance{
		Coins:   b.AsDecimal(),
		Credits: decimal.NewFromInt(-customer.Balance),
	}, nil
}
