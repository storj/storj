// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package piecetracker

// Config is the configuration for the piecetracker.
type Config struct {
	UseRangedLoop   bool `help:"whether to enable piece tracker observer with ranged loop" default:"true"`
	UpdateBatchSize int  `help:"batch size for updating nodes with number of pieces" default:"1000"`
}
