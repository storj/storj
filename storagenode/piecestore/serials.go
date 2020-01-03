// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"time"

	"storj.io/common/storj"
)

// SerialNumberFn is callback from IterateAll
type SerialNumberFn func(satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time)

// UsedSerials is a persistent store for serial numbers.
// TODO: maybe this should be in orders.UsedSerials
//
// architecture: Database
type UsedSerials interface {
	// Add adds a serial to the database.
	Add(ctx context.Context, satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) error
	// DeleteExpired deletes expired serial numbers
	DeleteExpired(ctx context.Context, now time.Time) error

	// IterateAll iterates all serials.
	// Note, this will lock the database and should only be used during startup.
	IterateAll(ctx context.Context, fn SerialNumberFn) error
}
