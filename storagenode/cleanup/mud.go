// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*SafeLoop](ball, NewSafeLoop)
	mud.Provide[*Cleanup](ball, NewCleanup)
	mud.Provide[*CoreLoad](ball, NewCoreLoad)
	mud.Provide[*FileExists](ball, NewFileExists)
	mud.Provide[*Period](ball, NewPeriod)
	mud.Provide[*DeleteEmpty](ball, NewDeleteEmpty)

	mud.RegisterImplementation[[]Enablement](ball)
	mud.Implementation[[]Enablement, *CoreLoad](ball)
	mud.Implementation[[]Enablement, *FileExists](ball)
	mud.Implementation[[]Enablement, *Period](ball)

	config.RegisterConfig[SafeLoopConfig](ball, "cleanup.loop")
	config.RegisterConfig[CoreLoadConfig](ball, "cleanup.load")
	config.RegisterConfig[FileExistsConfig](ball, "cleanup.file")
	config.RegisterConfig[PeriodConfig](ball, "cleanup.period")
	config.RegisterConfig[Config](ball, "cleanup")
}
