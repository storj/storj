// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tracing

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud Module definition for tracing.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "tracing")
	mud.Provide[*Tracing](ball, NewTracing)
}
