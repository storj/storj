package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

type UserCredits interface {
	TotalReferredCount(ctx context.Context, userID uuid.UUID) (int64, error)
	AvailableCredits(ctx context.Context, userID uuid.UUID, expirationEndDate time.Time) ([]UserCredit, error)
	Create(ctx context.Context, userCredit UserCredit) (*UserCredit, error)
	UpdateAvailableCredits(ctx context.Context, appliedCredits int, id uuid.UUID, billingStartDate time.Time) error
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
