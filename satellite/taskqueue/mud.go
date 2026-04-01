// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers taskqueue components.
func Module(ball *mud.Ball) {
	mud.Provide[*Client](ball, NewClient)
	mud.Provide[*Monitor](ball, NewMonitor)
	config.RegisterConfig[Config](ball, "taskqueue")
	config.RegisterConfig[RunnerConfig](ball, "taskqueue.worker")
	config.RegisterConfig[MonitorConfig](ball, "taskqueue.monitor")
}
