// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

var (
	// UnattachedErrString is part of the err string returned by stripe if a payment
	// method does not belong to a customer.
	UnattachedErrString = "The payment method must be attached to the customer"
)

// creditCards is an implementation of payments.CreditCards.
//
// architecture: Service
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

	cusParams := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}}
	customer, err := creditCards.service.stripeClient.Customers().Get(customerID, cusParams)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	cardParams := &stripe.PaymentMethodListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods().List(cardParams)
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
func (creditCards *creditCards) Add(ctx context.Context, userID uuid.UUID, cardToken string) (_ payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID, cardToken)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.CreditCard{}, payments.ErrAccountNotSetup.Wrap(err)
	}

	cardParams := &stripe.PaymentMethodParams{
		Params: stripe.Params{Context: ctx},
		Type:   stripe.String(string(stripe.PaymentMethodTypeCard)),
		Card:   &stripe.PaymentMethodCardParams{Token: &cardToken},
	}

	card, err := creditCards.service.stripeClient.PaymentMethods().New(cardParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return payments.CreditCard{}, Error.Wrap(err)
	}

	listParams := &stripe.PaymentMethodListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	now := time.Now()
	currentYear := int64(now.Year())
	currentMonth := int64(now.Month())

	var (
		count        int
		expiredCards []*stripe.PaymentMethod
	)

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods().List(listParams)
	for paymentMethodsIterator.Next() {
		count++
		stripeCard := paymentMethodsIterator.PaymentMethod()

		if stripeCard.Card.Fingerprint == card.Card.Fingerprint &&
			stripeCard.Card.ExpMonth == card.Card.ExpMonth &&
			stripeCard.Card.ExpYear == card.Card.ExpYear {
			return payments.CreditCard{}, payments.ErrDuplicateCard.New("this card is already on file for your account.")
		}

		if stripeCard.Card.ExpYear < currentYear || (stripeCard.Card.ExpYear == currentYear && stripeCard.Card.ExpMonth < currentMonth) {
			expiredCards = append(expiredCards, stripeCard)
		}
	}
	if err = paymentMethodsIterator.Err(); err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	if count >= creditCards.service.stripeConfig.MaxCreditCardCount {
		return payments.CreditCard{}, payments.ErrMaxCreditCards.New("you have reached the maximum number of credit cards for your account.")
	}

	attachParams := &stripe.PaymentMethodAttachParams{
		Params:   stripe.Params{Context: ctx},
		Customer: &customerID,
	}

	card, err = creditCards.service.stripeClient.PaymentMethods().Attach(card.ID, attachParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return payments.CreditCard{}, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(card.ID),
		},
	}

	_, err = creditCards.service.stripeClient.Customers().Update(customerID, params)
	if err != nil {
		creditCards.service.log.Warn("failed to make new card default", zap.String("user_id", userID.String()), zap.Error(err))
	}

	// We remove all expired cards from the customer only if the new card is successfully attached and marked as default.
	if err == nil && len(expiredCards) > 0 {
		detachParams := &stripe.PaymentMethodDetachParams{Params: stripe.Params{Context: ctx}}

		for _, expiredCard := range expiredCards {
			_, err = creditCards.service.stripeClient.PaymentMethods().Detach(expiredCard.ID, detachParams)
			if err != nil {
				creditCards.service.log.Warn("failed to detach expired credit card", zap.String("user_id", userID.String()), zap.Error(err))
			}
		}
	}

	return payments.CreditCard{
		ID:        card.ID,
		ExpMonth:  int(card.Card.ExpMonth),
		ExpYear:   int(card.Card.ExpYear),
		Brand:     string(card.Card.Brand),
		Last4:     card.Card.Last4,
		IsDefault: err == nil,
	}, nil
}

// Update updates the credit card details.
func (creditCards *creditCards) Update(ctx context.Context, userID uuid.UUID, params payments.CardUpdateParams) (err error) {
	defer mon.Task()(&ctx, userID, params.CardID)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.ErrAccountNotSetup.Wrap(err)
	}

	cardIter := creditCards.service.stripeClient.PaymentMethods().List(&stripe.PaymentMethodListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
	})

	isUserCard := false
	for cardIter.Next() {
		if cardIter.PaymentMethod().ID == params.CardID {
			isUserCard = true
			break
		}
	}

	if err = cardIter.Err(); err != nil {
		return Error.Wrap(err)
	}

	if !isUserCard {
		return payments.ErrCardNotFound.New("this card is not attached to this account.")
	}

	cardParams := &stripe.PaymentMethodParams{
		Params: stripe.Params{Context: ctx},
		Card: &stripe.PaymentMethodCardParams{
			ExpMonth: stripe.Int64(params.ExpMonth),
			ExpYear:  stripe.Int64(params.ExpYear),
		},
	}

	_, err = creditCards.service.stripeClient.PaymentMethods().Update(params.CardID, cardParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return Error.Wrap(err)
	}

	return nil
}

// AddByPaymentMethodID is used to save new credit card, attach it to payment account and make it default
// using the payment method id instead of the token. In this case, the payment method should already be
// created by the frontend using the stripe payment element for example.
func (creditCards *creditCards) AddByPaymentMethodID(ctx context.Context, userID uuid.UUID, pmID string, force bool) (_ payments.CreditCard, err error) {
	defer mon.Task()(&ctx, userID, pmID)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.CreditCard{}, payments.ErrAccountNotSetup.Wrap(err)
	}

	card, err := creditCards.service.stripeClient.PaymentMethods().Get(pmID, &stripe.PaymentMethodParams{
		Params: stripe.Params{Context: ctx},
	})
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	listParams := &stripe.PaymentMethodListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
	}

	now := time.Now()
	currentYear := int64(now.Year())
	currentMonth := int64(now.Month())

	var (
		count        int
		expiredCards []*stripe.PaymentMethod
	)

	paymentMethodsIterator := creditCards.service.stripeClient.PaymentMethods().List(listParams)
	for paymentMethodsIterator.Next() {
		count++
		stripeCard := paymentMethodsIterator.PaymentMethod()

		if stripeCard.Card.Fingerprint == card.Card.Fingerprint &&
			stripeCard.Card.ExpMonth == card.Card.ExpMonth &&
			stripeCard.Card.ExpYear == card.Card.ExpYear &&
			!force {
			return payments.CreditCard{}, payments.ErrDuplicateCard.New("this card is already on file for your account.")
		}

		if stripeCard.Card.ExpYear < currentYear || (stripeCard.Card.ExpYear == currentYear && stripeCard.Card.ExpMonth < currentMonth) {
			expiredCards = append(expiredCards, stripeCard)
		}
	}
	if err = paymentMethodsIterator.Err(); err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	if count >= creditCards.service.stripeConfig.MaxCreditCardCount {
		return payments.CreditCard{}, payments.ErrMaxCreditCards.New("you have reached the maximum number of credit cards for your account.")
	}

	attachParams := &stripe.PaymentMethodAttachParams{
		Params:   stripe.Params{Context: ctx},
		Customer: &customerID,
	}

	card, err = creditCards.service.stripeClient.PaymentMethods().Attach(pmID, attachParams)
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return payments.CreditCard{}, Error.Wrap(err)
	}

	params := &stripe.CustomerParams{
		Params: stripe.Params{Context: ctx},
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(card.ID),
		},
	}

	_, err = creditCards.service.stripeClient.Customers().Update(customerID, params)
	if err != nil {
		creditCards.service.log.Warn("failed to make new card default", zap.String("user_id", userID.String()), zap.Error(err))
	}

	// We remove all expired cards from the customer only if the new card is successfully attached and marked as default.
	if err == nil && len(expiredCards) > 0 {
		detachParams := &stripe.PaymentMethodDetachParams{Params: stripe.Params{Context: ctx}}

		for _, expiredCard := range expiredCards {
			_, err = creditCards.service.stripeClient.PaymentMethods().Detach(expiredCard.ID, detachParams)
			if err != nil {
				creditCards.service.log.Warn("failed to detach expired credit card", zap.String("user_id", userID.String()), zap.Error(err))
			}
		}
	}

	return payments.CreditCard{
		ID:        card.ID,
		ExpMonth:  int(card.Card.ExpMonth),
		ExpYear:   int(card.Card.ExpYear),
		Brand:     string(card.Card.Brand),
		Last4:     card.Card.Last4,
		IsDefault: err == nil,
	}, nil
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
		Params: stripe.Params{Context: ctx},
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			DefaultPaymentMethod: stripe.String(cardID),
		},
	}

	_, err = creditCards.service.stripeClient.Customers().Update(customerID, params)
	if err != nil && strings.Contains(err.Error(), UnattachedErrString) {
		return payments.ErrCardNotFound.New("this card is not attached to this account.")
	}

	return Error.Wrap(err)
}

// Remove is used to remove credit card from payment account.
// Setting force to true will allow to remove the default payment method.
func (creditCards *creditCards) Remove(ctx context.Context, userID uuid.UUID, cardID string, force bool) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	customerID, err := creditCards.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return payments.ErrAccountNotSetup.Wrap(err)
	}

	cusParams := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}}
	customer, err := creditCards.service.stripeClient.Customers().Get(customerID, cusParams)
	if err != nil {
		return Error.Wrap(err)
	}
	if customer.InvoiceSettings != nil &&
		customer.InvoiceSettings.DefaultPaymentMethod != nil &&
		customer.InvoiceSettings.DefaultPaymentMethod.ID == cardID &&
		!force {
		return payments.ErrDefaultCard.New("can not detach default payment method.")
	}

	cardIter := creditCards.service.stripeClient.PaymentMethods().List(&stripe.PaymentMethodListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   &customerID,
		Type:       stripe.String(string(stripe.PaymentMethodTypeCard)),
	})

	isUserCard := false
	for cardIter.Next() {
		if cardIter.PaymentMethod().ID == cardID {
			isUserCard = true
			break
		}
	}

	if err = cardIter.Err(); err != nil {
		return Error.Wrap(err)
	}

	if !isUserCard {
		return payments.ErrCardNotFound.New("this card is not attached to this account.")
	}

	cardParams := &stripe.PaymentMethodDetachParams{Params: stripe.Params{Context: ctx}}
	_, err = creditCards.service.stripeClient.PaymentMethods().Detach(cardID, cardParams)

	return Error.Wrap(err)
}

// RemoveAll is used to detach all credit cards from payment account.
// It should only be used in case of a user deletion. In case of an error, some cards could have been deleted already.
func (creditCards *creditCards) RemoveAll(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	ccList, err := creditCards.List(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	params := &stripe.PaymentMethodDetachParams{Params: stripe.Params{Context: ctx}}
	for _, cc := range ccList {
		_, err = creditCards.service.stripeClient.PaymentMethods().Detach(cc.ID, params)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// GetSetupSecret begins the process of setting up a card for payments with authorization
// by creating a setup intent. Returns a secret that can be used to complete the setup
// on the frontend.
func (creditCards *creditCards) GetSetupSecret(ctx context.Context) (secret string, err error) {
	defer mon.Task()(&ctx)(&err)

	intent, err := creditCards.service.stripeClient.SetupIntents().New(&stripe.SetupIntentParams{
		Params:             stripe.Params{Context: ctx},
		Usage:              stripe.String(string(stripe.SetupIntentUsageOffSession)),
		PaymentMethodTypes: stripe.StringSlice([]string{string(stripe.PaymentMethodTypeCard)}),
	})
	if err != nil {
		stripeErr := &stripe.Error{}
		if errors.As(err, &stripeErr) {
			err = errs.Wrap(errors.New(stripeErr.Msg))
		}
		return "", Error.Wrap(err)
	}

	return intent.ClientSecret, nil
}
