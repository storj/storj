package payments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// CouponsDB is an interface for managing coupons table.
//
// architecture: Database
type CouponsDB interface {
	// Insert inserts a stripe customer into the database.
	Insert(ctx context.Context, coupon Coupon) error
	// Insert inserts a stripe customer into the database.
	Update(ctx context.Context, coupon Coupon) error
	// Insert inserts a stripe customer into the database.
	Delete(ctx context.Context, couponID uuid.UUID) error
	// List returns page with customers ids created before specified date.
	List(ctx context.Context, offset int64, limit int, before time.Time) (error, error)
}

// Coupon is an entity that adds some funds to Accounts balance for some fixed period.
// At the end of the period, the entire remaining coupon amount will be returned from the account balance.
type Coupon struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"userId"`
	Amount          int64     `json:"amount"`
	RemainingAmount int64     `json:"remainingAmount"`
	Description     string    `json:"description"`
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
}
