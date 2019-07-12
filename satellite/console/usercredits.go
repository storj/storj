// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/internal/currency"
)

// UserCredits holds information to interact with database
type UserCredits interface {
	GetCreditUsage(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) (*UserCreditUsage, error)
	Create(ctx context.Context, userCredit UserCredit) error
	UpdateAvailableCredits(ctx context.Context, creditsToCharge int, id uuid.UUID, billingStartDate time.Time) (remainingCharge int, err error)
}

// UserCredit holds information about an user's credit
type UserCredit struct {
	ID            int
	UserID        uuid.UUID
	OfferID       int
	ReferredBy    *uuid.UUID
	CreditsEarned currency.USD
	CreditsUsed   currency.USD
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

// UserCreditUsage holds information about credit usage information
type UserCreditUsage struct {
	Referred         int64
	AvailableCredits currency.USD
	UsedCredits      currency.USD
}
