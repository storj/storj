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
	CardID string `json:"cardID"`
	Amount int    `json:"amount"`
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
