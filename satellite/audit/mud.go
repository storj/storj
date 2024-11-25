// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {

	mud.Provide[*Verifier](ball, NewVerifier)

	// TODO: we need real containment for running service.
	mud.Provide[Containment](ball, func() Containment {
		return &noContainment{}
	})
	mud.Provide[*RunOnce](ball, NewRunOnce)
	config.RegisterConfig[Config](ball, "audit")
	config.RegisterConfig[RunOnceConfig](ball, "audit")

}
