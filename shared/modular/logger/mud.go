// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package logger

import (
	"go.uber.org/zap"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module that provides a logger.
func Module(ball *mud.Ball) {
	mud.Provide[*zap.Config](ball, NewZapConfig)
	mud.Provide[RootLogger](ball, NewRootLogger)
	mud.Factory[*zap.Logger](ball, NewLogger)
	config.RegisterConfig[Config](ball, "log")
}
