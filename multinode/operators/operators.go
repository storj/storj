// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package operators

import (
	"storj.io/common/storj"
)

// Operator contains SNO payouts contact details and amount of undistributed payouts.
type Operator struct {
	NodeID         storj.NodeID `json:"nodeId"`
	Email          string       `json:"email"`
	Wallet         string       `json:"wallet"`
	WalletFeatures []string     `json:"walletFeatures"`
	Undistributed  int64        `json:"undistributed"`
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
