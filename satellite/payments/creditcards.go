// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// CreditCards exposes all needed functionality to manage account credit cards.
type CreditCards interface {
	// List returns a list of PaymentMethods for a given account.
	List(ctx context.Context, userID uuid.UUID) ([]CreditCard, error)

	// Add is used to save new credit card and attach it to payment account.
	Add(ctx context.Context, userID uuid.UUID, cardToken string) error
}

// CreditCard holds all public information about credit card.
type CreditCard struct {
	ID       []byte `json:"id"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
	Brand    string `json:"brand"`
	Last4    string `json:"last4"`
}
