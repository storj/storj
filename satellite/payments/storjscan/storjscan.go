// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

var mon = monkit.Package()

// Config stores needed information for storjscan service initialization.
type Config struct {
	Endpoint string `help:"storjscan API endpoint"`
	Auth     struct {
		Identifier string `help:"basic auth identifier"`
		Secret     string `help:"basic auth secret"`
	}
	Interval      time.Duration `help:"storjscan chore interval to query new payments for all satellite deposit wallets" default:"1m"`
	Confirmations int           `help:"required number of following blocks in the chain to accept payment as confirmed" default:"15"`
	DisableLoop   bool          `help:"flag to disable querying new storjscan payments by storjscan chore" default:"true"`
}
