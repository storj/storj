// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/logger"
	"storj.io/storj/storagenode"
)

// Module registers all the possible components for the storagenode instance.
func Module(ball *mud.Ball) {
	logger.Module(ball)
	modular.IdentityModule(ball)
	storagenode.Module(ball)
}
