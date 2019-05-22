// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

type PaymentMethod struct {
	Brand      string
	LastFour   string
}

type ProjectInvoice struct {
	ProjectID uuid.UUID
	InvoiceID string

	Status        string
	Amount        int64
	PaymentMethod PaymentMethod

	StartDate time.Time
	EndDate   time.Time

	DownloadLink string

	CreatedAt time.Time
}
