// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

var (
	mon = monkit.Package()
)

// Config for the chore.
type Config struct {
	DetectionInterval          time.Duration `help:"how often to run the downtime detection chore." releaseDefault:"1h0s" devDefault:"30s"`
	EstimationInterval         time.Duration `help:"how often to run the downtime estimation chore" releaseDefault:"1h0s" devDefault:"30s"`
	EstimationBatchSize        int           `help:"the downtime estimation chore should check this many offline nodes" releaseDefault:"1000" devDefault:"100"`
	EstimationConcurrencyLimit int           `help:"max number of concurrent connections in estimation chore" default:"10"`
}
