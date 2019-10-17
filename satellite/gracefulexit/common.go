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

	mon = monkit.Package()
)

// Config for the chore
type Config struct {
	ChoreBatchSize int           `help:"size of the buffer used to batch inserts into the transfer queue." default:"500"`
	ChoreInterval  time.Duration `help:"how often to run the transfer queue chore." releaseDefault:"30s" devDefault:"10s"`

	EndpointBatchSize   int `help:"size of the buffer used to batch transfer queue reads and sends to the storage node." default:"100"`
	EndpointMaxFailures int `help:"maximum number of transfer failures per piece." default:"3"`
}
