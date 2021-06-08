// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package operators

// Operator contains contains SNO payouts contact details.
type Operator struct {
	Email          string   `json:"email"`
	Wallet         string   `json:"wallet"`
	WalletFeatures []string `json:"walletFeatures"`
}

// Cursor holds operator cursor entity which is used to create listed page.
type Cursor struct {
	Limit int64
	Page  int64
}

// Page holds operator page entity which is used to show listed page of operators.
type Page struct {
	Operators   []Operator `json:"operators"`
	Offset      int64      `json:"offset"`
	Limit       int64      `json:"limit"`
	CurrentPage int64      `json:"currentPage"`
	PageCount   int64      `json:"pageCount"`
	TotalCount  int64      `json:"totalCount"`
}
