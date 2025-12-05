// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package signaturecheck

import (
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module registers the signature checking components with the mud.Ball.
func Module(ball *mud.Ball) {
	mud.Provide[*Full](ball, func() *Full {
		return &Full{}
	})
	mud.Provide[*AcceptAll](ball, func() *AcceptAll {
		return &AcceptAll{}
	})
	config.RegisterConfig[Config](ball, "signature-check")
	mud.Provide[*Trusted](ball, NewTrusted)
	mud.RegisterInterfaceImplementation[Check, *Full](ball)
}
