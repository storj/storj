// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// UserPaymentInfos is user payment infos store
type UserPaymentInfos interface {
	Create(ctx context.Context, info UserPaymentInfo) (*UserPaymentInfo, error)
	Get(ctx context.Context, userID uuid.UUID) (*UserPaymentInfo, error)
}

// UserPaymentInfo represents user payment information
type UserPaymentInfo struct {
	UserID     uuid.UUID
	CustomerID string

	CreatedAt time.Time
}
