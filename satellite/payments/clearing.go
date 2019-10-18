// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import "context"

// Clearing exposes control over payments clearing loop.
type Clearing interface {
	// Run runs payments clearing loop.
	Run(ctx context.Context) error
	// Closes closes payments clearing loop.
	Close() error
}
