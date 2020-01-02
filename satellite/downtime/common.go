// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package downtime

import (
	"time"

	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
)

// Config for the chore
type Config struct {
	DetectionInterval time.Duration `help:"how often to run the downtime detection chore." releaseDefault:"1h0s" devDefault:"30s"`
}
