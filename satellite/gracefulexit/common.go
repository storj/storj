// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	// Error is the default error class for graceful exit package.
	Error = errs.Class("gracefulexit")

	// ErrNodeNotFound is returned if a graceful exit entry for a  node does not exist in database
	ErrNodeNotFound = errs.Class("graceful exit node not found")

	mon = monkit.Package()
)

// Config for the chore
type Config struct {
	Enabled bool `help:"whether or not graceful exit is enabled on the satellite side." releaseDefault:"false" devDefault:"true"`

	ChoreBatchSize int           `help:"size of the buffer used to batch inserts into the transfer queue." default:"500"`
	ChoreInterval  time.Duration `help:"how often to run the transfer queue chore." releaseDefault:"30s" devDefault:"10s"`

	EndpointBatchSize int `help:"size of the buffer used to batch transfer queue reads and sends to the storage node." default:"100"`

	MaxFailuresPerPiece          int           `help:"maximum number of transfer failures per piece." default:"3"`
	OverallMaxFailuresPercentage int           `help:"maximum percentage of transfer failures per node." default:"10"`
	MaxInactiveTimeFrame         time.Duration `help:"maximum inactive time frame of transfer activities per node." default:"500h"`
	RecvTimeout                  time.Duration `help:"the minimum duration for receiving a stream from a storage node before timing out" default:"10m"`
}
