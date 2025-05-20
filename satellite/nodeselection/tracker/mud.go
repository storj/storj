// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tracker

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*PrometheusTracker](ball, NewPrometheusTracker)
	config.RegisterConfig[PrometheusTrackerConfig](ball, "prometheus-tracker")
}
