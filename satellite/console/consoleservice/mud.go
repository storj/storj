// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleservice

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, func(log *zap.Logger, db console.DB, accountFreezeService *console.AccountFreezeService, cfg console.Config) (*Service, error) {
		return NewService(log, ServiceDependencies{
			ConsoleDB:            db,
			AccountFreezeService: accountFreezeService,
		}, cfg)
	})
}
