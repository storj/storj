// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"time"
)

// BillingHistoryItem holds all public information about billing history line.
type BillingHistoryItem struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Amount      int64                  `json:"amount"`
	Received    int64                  `json:"received"`
	Status      string                 `json:"status"`
	Link        string                 `json:"link"`
	Start       time.Time              `json:"start"`
	End         time.Time              `json:"end"`
	Type        BillingHistoryItemType `json:"type"`
}

// BillingHistoryItemType indicates type of billing history item.
type BillingHistoryItemType int

const (
	// Invoice is a Stripe invoice billing item.
	Invoice BillingHistoryItemType = 0
	// Transaction is a Coinpayments transaction billing item.
	Transaction BillingHistoryItemType = 1
	// Charge is a credit card charge billing item.
	Charge BillingHistoryItemType = 2
)
