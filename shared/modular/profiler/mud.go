// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package profiler

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers all the possible components for the profiler instance.
func Module(ball *mud.Ball) {
	mud.Provide[*Profiler](ball, NewProfiler)
	config.RegisterConfig[Config](ball, "profiler")
}
