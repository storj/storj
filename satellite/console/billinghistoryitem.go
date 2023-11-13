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
	Remaining   int64                  `json:"remaining"`
	Received    int64                  `json:"received"`
	Status      string                 `json:"status"`
	Link        string                 `json:"link"`
	Start       time.Time              `json:"start"`
	End         time.Time              `json:"end"`
	Type        BillingHistoryItemType `json:"type"`
}

// BillingHistoryCursor holds info for billing history
// cursor pagination.
type BillingHistoryCursor struct {
	Limit int

	// StartingAfter is the last ID of the previous page.
	// The next page will start after this ID.
	StartingAfter string
	// EndingBefore is the id before which a page should end.
	EndingBefore string
}

// BillingHistoryPage returns paginated billing history items.
type BillingHistoryPage struct {
	Items []BillingHistoryItem `json:"items"`
	// Next indicates whether there are more events to retrieve.
	Next bool `json:"next"`
	// Previous indicates whether there are previous items.
	Previous bool `json:"previous"`
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
	// Coupon is an entity that adds some funds to Accounts balance for some fixed period.
	Coupon BillingHistoryItemType = 3
	// DepositBonus is an entity that adds some funds to Accounts balance after deposit with storj coins.
	DepositBonus BillingHistoryItemType = 4
)
