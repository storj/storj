// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import "time"

type Config struct {
	ReportsInterval time.Duration `help:"amount of time we wait before running next send reports interval" devDefault:"1m" releaseDefault:"24h"`
}
