// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// UserPayments is user payment infos store
type UserPayments interface {
	Create(ctx context.Context, info UserPayment) (*UserPayment, error)
	Get(ctx context.Context, userID uuid.UUID) (*UserPayment, error)
}

// UserPayment represents user payment information
type UserPayment struct {
	UserID     uuid.UUID
	CustomerID []byte

	CreatedAt time.Time
}
