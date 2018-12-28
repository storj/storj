// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/datarepair/irreparable"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/storage"
)

// DB is the master database for the satellite
type DB interface {
	BandwidthAgreement() bwagreement.DB
	// PointerDB() pointerdb.DB
	StatDB() statdb.DB
	OverlayCache() storage.KeyValueStore
	RepairQueue() queue.RepairQueue
	Accounting() accounting.DB
	Irreparable() irreparable.DB

	CreateTables() error
	Close() error
}
