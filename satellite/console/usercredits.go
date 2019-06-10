package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

type UserCredits interface {
	TotalRefferedCount(ctx context.Context, userID uuid.UUID) (int64, error)
	AvailableCredits(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) ([]UserCredit, error)
	Create(ctx context.Context, userCredit UserCredit) error
	UpdateAvailableCredits(ctx context.Context, appliedCredits int64, id uuid.UUID, expirationEndDate time.Time) error
}

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
