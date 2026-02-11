// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "accounting")
	mud.View[*Service, Service](ball, mud.Dereference[Service])

	mud.Provide[RetentionRemainderRecorderConfig](ball, func(config Config) RetentionRemainderRecorderConfig {
		return config.RetentionRemainderRecorder
	})

	mud.Provide[*RemainderChargeRecorder](ball, NewRemainderChargeRecorder)
}
