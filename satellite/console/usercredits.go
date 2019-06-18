// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// UserCredits holds information to interact with database
type UserCredits interface {
	TotalReferredCount(ctx context.Context, userID uuid.UUID) (int64, error)
	GetAvailableCredits(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) (int, error)
	GetUsedCredits(ctx context.Context, userID uuid.UUID) (total int, err error)
	Create(ctx context.Context, userCredit UserCredit) (*UserCredit, error)
	UpdateAvailableCredits(ctx context.Context, creditsToCharge int, id uuid.UUID, billingStartDate time.Time) (remainingCharge int, err error)
}

// UserCredit holds information about an user's credit
type UserCredit struct {
	ID                   int
	UserID               uuid.UUID
	OfferID              int
	ReferredBy           uuid.UUID
	CreditsEarnedInCents int
	CreditsUsedInCents   int
	ExpiresAt            time.Time
	CreatedAt            time.Time
}

type UserCreditsUsage struct {
	Referred         int64
	AvailableCredits int
	UsedCredits      int
}
