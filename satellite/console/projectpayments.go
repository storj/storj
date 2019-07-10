// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectPayments is project payment infos store interface
type ProjectPayments interface {
	Create(ctx context.Context, info ProjectPayment) (*ProjectPayment, error)
	Update(ctx context.Context, info ProjectPayment) error
	Delete(ctx context.Context, projectPaymentID uuid.UUID) error
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]*ProjectPayment, error)
	GetByID(ctx context.Context, projectPaymentID uuid.UUID) (*ProjectPayment, error)
	GetDefaultByProjectID(ctx context.Context, projectID uuid.UUID) (*ProjectPayment, error)
	GetByPayerID(ctx context.Context, payerID uuid.UUID) ([]*ProjectPayment, error)
}

// ProjectPayment contains project payment info
type ProjectPayment struct {
	ID uuid.UUID

	ProjectID uuid.UUID
	PayerID   uuid.UUID

	PaymentMethodID []byte
	Card            Card
	IsDefault       bool

	CreatedAt time.Time
}

// Card contains customer card info
type Card struct {
	Country         string
	Brand           string
	Name            string
	ExpirationMonth int64
	ExpirationYear  int64
	LastFour        string
}
