// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// PaymentMethod holds credit card display information
type PaymentMethod struct {
	ExpYear    uint64
	ExpMonth   uint64
	Brand      string
	LastFour   string
	HolderName string
	AddedAt    time.Time
}

// ProjectInvoice holds invoice general information
type ProjectInvoice struct {
	Number    string
	ProjectID uuid.UUID

	Status        string
	Amount        int64
	PaymentMethod PaymentMethod

	StartDate time.Time
	EndDate   time.Time

	DownloadLink string

	CreatedAt time.Time
}
