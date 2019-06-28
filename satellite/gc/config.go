// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import "time"

// Config contains configurable values for garbage collection
type Config struct {
	Interval time.Duration `help:"how frequently garbage collection filters should be sent to storage nodes" releaseDefault:"168h" devDefault:"168h"`
	Active   bool          `help:"set if garbage collection is actively running or not" releaseDefault:"true" devDefault:"true"`

	// TODO: find out what initial number of pieces should be when creating a filter
	InitialPieces     int64   `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"" devDefault:""`
	FalsePositiveRate float64 `help:"the false positive rate used for creating a filter" releaseDefault:"" devDefault:""`
}
