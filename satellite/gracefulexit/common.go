// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
)

var (
	// Error is the default error class for graceful exit package.
	Error = errs.Class("gracefulexit")

	// ErrNodeNotFound is returned if a graceful exit entry for a  node does not exist in database.
	ErrNodeNotFound = errs.Class("graceful exit node not found")

	// ErrAboveOptimalThreshold is returned if a graceful exit entry for a node has more pieces than required.
	ErrAboveOptimalThreshold = errs.Class("segment has more pieces than required")

	mon = monkit.Package()
)

// Config for the chore.
type Config struct {
	Enabled bool `help:"whether or not graceful exit is enabled on the satellite side." default:"true"`

	ChoreBatchSize int           `help:"size of the buffer used to batch inserts into the transfer queue." default:"500" testDefault:"10"`
	ChoreInterval  time.Duration `help:"how often to run the transfer queue chore." releaseDefault:"30s" devDefault:"10s" testDefault:"$TESTINTERVAL"`
	UseRangedLoop  bool          `help:"whether use GE observer with ranged loop." default:"true"`

	EndpointBatchSize int `help:"size of the buffer used to batch transfer queue reads and sends to the storage node." default:"300" testDefault:"100"`

	MaxFailuresPerPiece          int           `help:"maximum number of transfer failures per piece." default:"5"`
	OverallMaxFailuresPercentage int           `help:"maximum percentage of transfer failures per node." default:"10"`
	MaxInactiveTimeFrame         time.Duration `help:"maximum inactive time frame of transfer activities per node." default:"168h" testDefault:"10s"`
	RecvTimeout                  time.Duration `help:"the minimum duration for receiving a stream from a storage node before timing out" default:"2h" testDefault:"1m"`
	MaxOrderLimitSendCount       int           `help:"maximum number of order limits a satellite sends to a node before marking piece transfer failed" default:"10" testDefault:"3"`
	NodeMinAgeInMonths           int           `help:"minimum age for a node on the network in order to initiate graceful exit" default:"6" testDefault:"0"`

	AsOfSystemTimeInterval time.Duration `help:"interval for AS OF SYSTEM TIME clause (crdb specific) to read from db at a specific time in the past" default:"-10s" testDefault:"-1µs"`
	TransferQueueBatchSize int           `help:"batch size (crdb specific) for deleting and adding items to the transfer queue" default:"1000"`
}
