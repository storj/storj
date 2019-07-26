// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments"
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

// UserPaymentMethod represents user payment information
type UserPaymentMethod struct {
	UserID        uuid.UUID
	PaymentMethod payments.PaymentMethod
}

type UserPaymentMethodCombined struct {
	ID         string
	ExpYear    int64
	ExpMonth   int64
	CardBrand  string
	LastFour   string
	HolderName string
	AddedAt    time.Time
}