// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import "github.com/spacemonkeygo/monkit/v3"

var mon = monkit.Package()

// Config stores needed information for storjscan service initialization.
type Config struct {
	Endpoint string `help:"storjscan API endpoint"`
	Auth     struct {
		Identifier string `help:"basic auth identifier"`
		Secret     string `help:"basic auth secret"`
	}
	Confirmations int `help:"required number of following blocks in the chain to accept payment as confirmed" default:"12"`
}
