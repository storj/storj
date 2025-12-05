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
	// Create creates a new abstract payment intent.
	Create(ctx context.Context, req CreateIntentParams) (string, error)
}

// AddFundsParams holds the parameters needed to add funds to an account balance.
type AddFundsParams struct {
	CardID string           `json:"cardID"`
	Amount int              `json:"amount"`
	Intent ChargeCardIntent `json:"intent"` // Intent of the charge, e.g., AddFundsIntent or UpgradeAccountIntent
}

// CreateIntentParams holds the parameters needed to create a payment intent.
type CreateIntentParams struct {
	UserID         uuid.UUID
	Amount         int64
	Metadata       map[string]string
	WithCustomCard bool // Indicates if the intent should be created for processing a custom card.
}

// ChargeCardRequest is the request to charge a credit card.
type ChargeCardRequest struct {
	CardID string
	CreateIntentParams
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
)

// String returns the string representation of the ChargeCardIntent.
func (cci ChargeCardIntent) String() string {
	switch cci {
	case AddFundsIntent:
		return "add_funds"
	default:
		return "unknown_intent"
	}
}

// PurchaseIntent represents the intent of a purchase operation.
type PurchaseIntent int

const (
	// PurchasePackageIntent is used when the purchase is for a package plan.
	PurchasePackageIntent PurchaseIntent = 1
	// PurchaseUpgradedAccountIntent is used when the purchase is for upgrading an account.
	PurchaseUpgradedAccountIntent PurchaseIntent = 2
)
