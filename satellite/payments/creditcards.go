// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

var (
	// ErrCardNotFound is returned when card is not found for a user.
	ErrCardNotFound = errs.Class("card not found")
	// ErrDefaultCard is returned when a user tries to delete their default card.
	ErrDefaultCard = errs.Class("default card")
	// ErrDuplicateCard is returned when a user tries to add duplicate card.
	ErrDuplicateCard = errs.Class("duplicate card")
	// ErrMaxCreditCards is returned when a user tries to add more than the allowed number of credit cards.
	ErrMaxCreditCards = errs.Class("credit cards count")
)

// CreditCards exposes all needed functionality to manage account credit cards.
//
// architecture: Service
type CreditCards interface {
	// List returns a list of credit cards for a given payment account.
	List(ctx context.Context, userID uuid.UUID) ([]CreditCard, error)

	// Add is used to save new credit card and attach it to payment account.
	Add(ctx context.Context, userID uuid.UUID, cardToken string) (CreditCard, error)

	// Update updates the credit card details.
	Update(ctx context.Context, userID uuid.UUID, params CardUpdateParams) error

	// AddByPaymentMethodID is used to save new credit card, attach it to payment account and make it default
	// using the payment method id instead of the token. In this case, the payment method should already be
	// created by the frontend using stripe elements for example.
	AddByPaymentMethodID(ctx context.Context, userID uuid.UUID, pmID string, force bool) (CreditCard, error)

	// Remove is used to detach a credit card from payment account.
	Remove(ctx context.Context, userID uuid.UUID, cardID string, force bool) error

	// RemoveAll is used to detach all credit cards from payment account.
	// It should only be used in case of a user deletion.
	RemoveAll(ctx context.Context, userID uuid.UUID) error

	// MakeDefault makes a credit card default payment method.
	// this credit card should be attached to account before make it default.
	MakeDefault(ctx context.Context, userID uuid.UUID, cardID string) error

	// GetSetupSecret begins the process of setting up a card for payments with authorization
	// by creating a setup intent. Returns a secret that can be used to complete the setup
	// on the frontend.
	GetSetupSecret(ctx context.Context) (secret string, err error)
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

// CardUpdateParams holds the parameters needed to update a credit card.
type CardUpdateParams struct {
	CardID   string
	ExpMonth int64
	ExpYear  int64
}
