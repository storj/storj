// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"storj.io/common/uuid"
)

// PaymentIntents exposes all needed functionality to manage credit cards charging.
//
// architecture: Service
type PaymentIntents interface {
	// ChargeCard attempts to charge a credit card.
	ChargeCard(ctx context.Context, req ChargeCardRequest) (*ChargeCardResponse, error)
}

// AddFundsParams holds the parameters needed to add funds to an account balance.
type AddFundsParams struct {
	CardID string           `json:"cardID"`
	Amount int              `json:"amount"`
	Intent ChargeCardIntent `json:"intent"` // Intent of the charge, e.g., AddFundsIntent or UpgradeAccountIntent
}

// ChargeCardRequest is the request to charge a credit card.
type ChargeCardRequest struct {
	UserID   uuid.UUID
	CardID   string
	Amount   int64
	Metadata map[string]string
}

// ChargeCardResponse is the response to a charge request.
type ChargeCardResponse struct {
	Success         bool   `json:"success"`
	ClientSecret    string `json:"clientSecret"`
	PaymentIntentID string `json:"paymentIntentID"`
}

// ChargeCardIntent represents the intent of a charge card operation.
type ChargeCardIntent int

const (
	// AddFundsIntent is used when the charge is for adding funds to an account balance.
	AddFundsIntent ChargeCardIntent = 1
	// UpgradeAccountIntent is used when the charge is for upgrading an account.
	UpgradeAccountIntent ChargeCardIntent = 2
)

// String returns the string representation of the ChargeCardIntent.
func (cci ChargeCardIntent) String() string {
	switch cci {
	case AddFundsIntent:
		return "add_funds"
	case UpgradeAccountIntent:
		return "upgrade_account"
	default:
		return "unknown_intent"
	}
}
