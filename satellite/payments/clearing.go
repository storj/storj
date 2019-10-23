// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import "context"

// Clearing runs process of reconciling transactions deposits,
// customer balance, invoices and usages.
type Clearing interface {
	// Run runs payments clearing loop.
	Run(ctx context.Context) error
	// Closes closes payments clearing loop.
	Close() error
}
