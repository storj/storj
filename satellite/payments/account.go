// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
)

// Accounts exposes all needed functionality to manage payment accounts.
type Accounts interface {
	// Setup creates a payment account for the user.
	Setup(ctx context.Context, email string) error

	// Balance returns an integer amount in cents that represents the current balance of payment account.
	Balance(ctx context.Context) (int64, error)

	// CreditCards exposes all needed functionality to manage account credit cards.
	CreditCards() CreditCards
}
