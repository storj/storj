// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import "time"

// CardInfo holds information about credit card used for charge.
type CardInfo struct {
	ID       string `json:"id"`
	Brand    string `json:"brand"`
	LastFour string `json:"lastFour"`
}

// Charge contains charge details.
type Charge struct {
	ID        string    `json:"id"`
	Amount    int64     `json:"amount"`
	CardInfo  CardInfo  `json:"card"`
	CreatedAt time.Time `json:"createdAt"`
}
