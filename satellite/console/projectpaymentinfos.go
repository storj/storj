// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectPaymentInfos is project payment infos store interface
type ProjectPaymentInfos interface {
	Create(ctx context.Context, info ProjectPaymentInfo) (*ProjectPaymentInfo, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*ProjectPaymentInfo, error)
	GetByPayerID(ctx context.Context, payerID uuid.UUID) (*ProjectPaymentInfo, error)
}

// ProjectPaymentInfo contains project payment info
type ProjectPaymentInfo struct {
	ProjectID uuid.UUID
	PayerID   uuid.UUID

	PaymentMethodID []byte

	CreatedAt time.Time
}
