// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package orders

import (
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	mud.View[*overlay.Service, Overlay](ball, func(overlay *overlay.Service) Overlay {
		return overlay
	})
	config.RegisterConfig[Config](ball, "orders")
}
