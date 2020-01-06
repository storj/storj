// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/memory"
)

var (
	// Error is the default error class for graceful exit package.
	Error = errs.Class("gracefulexit")

	mon = monkit.Package()
)

// Config for graceful exit
type Config struct {
	ChoreInterval          time.Duration `help:"how often to run the chore to check for satellites for the node to exit." releaseDefault:"15m" devDefault:"10s"`
	NumWorkers             int           `help:"number of workers to handle satellite exits" default:"3"`
	NumConcurrentTransfers int           `help:"number of concurrent transfers per graceful exit worker" default:"1"`
	MinBytesPerSecond      memory.Size   `help:"the minimum acceptable bytes that an exiting node can transfer per second to the new node" default:"128B"`
	MinDownloadTimeout     time.Duration `help:"the minimum duration for downloading a piece from storage nodes before timing out" default:"2m"`
}
