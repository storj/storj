// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import "context"

// Observer processes a billing transaction.
type Observer interface {
	// Process is called repeatedly for each transaction.
	Process(context.Context, Transaction) error
}
