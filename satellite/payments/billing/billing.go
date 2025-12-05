// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

var mon = monkit.Package()

// Config stores needed information for billing service initialization.
type Config struct {
	Interval    time.Duration `help:"billing chore interval to query for new transactions from all payment types" default:"15s"`
	DisableLoop bool          `help:"flag to disable querying for new billing transactions by billing chore" default:"true"`
}
