// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stripe/stripe-go"

	"storj.io/storj/satellite/payments"
)

// creditCards is an implementation of payments.CreditCards.
type creditCards struct {
	service *Service
}

// List returns a list of credit cards for a given payment account.
func (creditCards *creditCards) List(ctx context.Context, userID uuid.UUID) (cards []payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	customer, err := creditCards.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	params := &stripe.PaymentMethodListParams{
		Customer: &customerID,
		Type:     stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods.List(params)
	for paymentMethodsIterator.Next() {
		stripeCard := paymentMethodsIterator.PaymentMethod()

		isDefault := false
		if customer.InvoiceSettings.DefaultPaymentMethod != nil {
			isDefault = customer.InvoiceSettings.DefaultPaymentMethod.ID == stripeCard.ID
		}

		cards = append(cards, payments.CreditCard{
			ID:        stripeCard.ID,
			ExpMonth:  int(stripeCard.Card.ExpMonth),
			ExpYear:   int(stripeCard.Card.ExpYear),
			Brand:     string(stripeCard.Card.Brand),
			Last4:     stripeCard.Card.Last4,
			IsDefault: isDefault,
		})
	}

	if err = paymentMethodsIterator.Err(); err != nil {
		return nil, Error.Wrap(err)
	}

	return cards, nil
}

// Add is used to save new credit card, attach it to payment account and make it default.
func (creditCards *creditCards) Add(ctx context.Context, userID uuid.UUID, cardToken string) (err error) {
	defer mon.Task()(&ctx, userID, cardToken)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.ErrAccountNotSetup.Wrap(err)
	}

	cardParams := &stripe.PaymentMethodParams{
		Type: stripe.String(string(stripe.PaymentMethodTypeCard)),
		Card: &stripe.PaymentMethodCardParams{Token: &cardToken},
	}

	card, err := creditCards.service.stripeClient.PaymentMethods.New(cardParams)
	if err != nil {
		return Error.Wrap(err)
	}

	attachParams := &stripe.PaymentMethodAttachParams{
		Customer: &customerID,
	}

	card, err = creditCards.service.stripeClient.PaymentMethods.Attach(card.ID, attachParams)
	if err != nil {
		return Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(card.ID),
		},
	}

	_, err = creditCards.service.stripeClient.Customers.Update(customerID, params)

	// TODO: handle created but not attached card manually?
	return Error.Wrap(err)
}

// MakeDefault makes a credit card default payment method.
// this credit card should be attached to account before make it default.
func (creditCards *creditCards) MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, userID, cardID)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.ErrAccountNotSetup.Wrap(err)
	}

	params := &stripe.CustomerParams{
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(cardID),
		},
	}

	_, err = creditCards.service.stripeClient.Customers.Update(customerID, params)

	return Error.Wrap(err)
}

// Remove is used to remove credit card from payment account.
func (creditCards *creditCards) Remove(ctx context.Context, userID uuid.UUID, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	_, err = creditCards.service.stripeClient.PaymentMethods.Detach(cardID, nil)

	return Error.Wrap(err)
}
