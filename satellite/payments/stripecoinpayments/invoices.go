// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

var ErrInvoiceRecordExists = Error.New("project invoice record already exists")

type InvoicesDB interface {
	RecordProjectInvoicing(ctx context.Context, projectID uuid.UUID, start, before time.Time) error
	CheckProjectInvoicing(ctx context.Context, projectID uuid.UUID, start, before time.Time) error
}
