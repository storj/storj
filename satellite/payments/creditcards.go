// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"storj.io/common/uuid"
)

// CreditCards exposes all needed functionality to manage account credit cards.
//
// architecture: Service
type CreditCards interface {
	// List returns a list of credit cards for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]CreditCard, error)

	// Add is used to save new credit card and attach it to payment account.
	Add(ctx context.Context, userID uuid.UUID, cardToken string) (CreditCard, error)

	// AddByPaymentMethodID is used to save new credit card, attach it to payment account and make it default
	// using the payment method id instead of the token. In this case, the payment method should already be
	// created by the frontend using stripe elements for example.
	AddByPaymentMethodID(ctx context.Context, userID uuid.UUID, pmID string) (CreditCard, error)

	// Remove is used to detach a credit card from payment account.
	Remove(ctx context.Context, userID uuid.UUID, cardID string) error

	// RemoveAll is used to detach all credit cards from payment account.
	// It should only be used in case of a user deletion.
	RemoveAll(ctx context.Context, userID uuid.UUID) error

	// MakeDefault makes a credit card default payment method.
	// this credit card should be attached to account before make it default.
	MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) error
}

// CreditCard holds all public information about credit card.
type CreditCard struct {
	ID        string `json:"id"`
	ExpMonth  int    `json:"expMonth"`
	ExpYear   int    `json:"expYear"`
	Brand     string `json:"brand"`
	Last4     string `json:"last4"`
	IsDefault bool   `json:"isDefault"`
}
