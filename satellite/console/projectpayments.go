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
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*ProjectPayment, error)
	GetDefaultByProjctID(ctx context.Context, projectID uuid.UUID)(*ProjectPayment, error)
	GetByPayerID(ctx context.Context, payerID uuid.UUID) (*ProjectPayment, error)
}

// ProjectPayment contains project payment info
type ProjectPayment struct {
	ProjectID uuid.UUID
	PayerID   uuid.UUID

	PaymentMethodID []byte

	CreatedAt time.Time
}
