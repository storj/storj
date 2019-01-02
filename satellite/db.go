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
	// CreateTables initializes the database
	CreateTables() error
	// Close closes the database
	Close() error

	// BandwidthAgreement returns database for storing bandwidth agreements
	BandwidthAgreement() bwagreement.DB
	// StatDB returns database for storing node statistics
	StatDB() statdb.DB
	// OverlayCache returns database for caching overlay information
	OverlayCache() storage.KeyValueStore
	// Accounting returns database for storing information about data use
	Accounting() accounting.DB
	// RepairQueue returns queue for segments that need repairing
	RepairQueue() queue.RepairQueue
	// Irreparable returns database for failed repairs
	Irreparable() irreparable.DB
}
